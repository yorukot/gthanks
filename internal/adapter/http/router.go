package http

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"log/slog"
	nethttp "net/http"
	"strconv"
	"strings"
	"time"

	"gthanks/internal/adapter/http/middleware"
	"gthanks/internal/adapter/imagegrid"
	"gthanks/internal/config"
	"gthanks/internal/domain"
	"gthanks/internal/usecase"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

//go:embed static/index.html
var indexHTML string

const backgroundImageJobTimeout = 2 * time.Minute

type imageJobResult struct {
	response domain.ContributionResponse
	pngBytes []byte
	err      error
}

func NewRouter(cfg config.Config, logger *slog.Logger, service *usecase.Service) nethttp.Handler {
	renderer := imagegrid.NewRenderer(service.Store())

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(cfg.RequestTimeout))
	r.Use(middleware.RequestLogger(logger))

	r.Get("/", indexHandler())
	r.Get("/healthz", func(w nethttp.ResponseWriter, _ *nethttp.Request) {
		writeJSON(w, nethttp.StatusOK, map[string]any{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})
	r.Get("/v1/contributions", contributionsHandler(service))
	r.Get("/v1/contributions/image", contributionsImageHandler(cfg, service, renderer))

	return r
}

func indexHandler() nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, _ *nethttp.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(nethttp.StatusOK)
		_, _ = w.Write([]byte(indexHTML))
	}
}

func contributionsHandler(service *usecase.Service) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
		includeBots, includeBotsSet, err := optionalBool(r, "include_bots")
		if err != nil {
			writeError(w, nethttp.StatusBadRequest, "invalid_query", "include_bots must be a boolean")
			return
		}
		if !includeBotsSet {
			includeBots = true
		}

		input := usecase.GetContributionsInput{
			Target:         r.URL.Query().Get("target"),
			Refresh:        r.URL.Query().Get("refresh") == "true",
			Summary:        true,
			IncludeForks:   r.URL.Query().Get("include_forks") == "true",
			IncludeBots:    includeBots,
			IncludeBotsSet: true,
		}
		if raw := r.URL.Query().Get("summary"); raw != "" {
			parsed, err := strconv.ParseBool(raw)
			if err != nil {
				writeError(w, nethttp.StatusBadRequest, "invalid_query", "summary must be a boolean")
				return
			}
			input.Summary = parsed
		}

		response, err := service.GetContributions(r.Context(), input)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		w.Header().Set("X-Cache-Status", response.Cache.Status)
		w.Header().Set("X-GitHub-Requests", strconv.Itoa(response.GitHubRequestCount))
		writeJSON(w, nethttp.StatusOK, response)
	}
}

func contributionsImageHandler(cfg config.Config, service *usecase.Service, renderer *imagegrid.Renderer) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
		includeBots, includeBotsSet, err := optionalBool(r, "include_bots")
		if err != nil {
			writeError(w, nethttp.StatusBadRequest, "invalid_query", "include_bots must be a boolean")
			return
		}
		if !includeBotsSet {
			includeBots = true
		}

		input := usecase.GetContributionsInput{
			Target:         strings.TrimSpace(r.URL.Query().Get("target")),
			Refresh:        r.URL.Query().Get("refresh") == "true",
			Summary:        true,
			IncludeForks:   r.URL.Query().Get("include_forks") == "true",
			IncludeBots:    includeBots,
			IncludeBotsSet: true,
		}

		perRow, err := optionalInt(r, "per_row")
		if err != nil {
			writeError(w, nethttp.StatusBadRequest, "invalid_query", "per_row must be an integer")
			return
		}
		width, err := optionalInt(r, "width")
		if err != nil {
			writeError(w, nethttp.StatusBadRequest, "invalid_query", "width must be an integer")
			return
		}
		limit, err := optionalInt(r, "limit")
		if err != nil {
			writeError(w, nethttp.StatusBadRequest, "invalid_query", "limit must be an integer")
			return
		}
		padding, err := optionalInt(r, "padding")
		if err != nil {
			writeError(w, nethttp.StatusBadRequest, "invalid_query", "padding must be an integer")
			return
		}
		space, spaceSet, err := optionalIntWithPresence(r, "space")
		if err != nil {
			writeError(w, nethttp.StatusBadRequest, "invalid_query", "space must be an integer")
			return
		}

		target, err := domain.ParseTarget(input.Target)
		if err != nil {
			writeDomainError(w, err)
			return
		}

		imageOptions := imagegrid.Options{
			PerRow:   perRow,
			Width:    width,
			Shape:    imagegrid.Shape(strings.ToLower(strings.TrimSpace(r.URL.Query().Get("shape")))),
			Limit:    limit,
			Padding:  padding,
			Space:    space,
			SpaceSet: spaceSet,
		}
		imageCacheKey, err := buildImageCacheKey(target, imageOptions, input.IncludeForks, input.IncludeBots)
		if err != nil {
			writeError(w, nethttp.StatusBadRequest, "invalid_image_options", err.Error())
			return
		}

		if !input.Refresh {
			cachedImage, err := service.GetImageCache(r.Context(), imageCacheKey)
			if err != nil {
				writeError(w, nethttp.StatusInternalServerError, "internal_error", err.Error())
				return
			}
			if cachedImage != nil && cachedImage.ExpiresAt.After(time.Now().UTC()) {
				w.Header().Set("Content-Type", "image/png")
				w.Header().Set("X-Image-Cache-Status", "hit")
				_, _ = w.Write(cachedImage.Content)
				return
			}
		}

		detachedCtx, cancel := context.WithTimeout(context.Background(), backgroundImageJobTimeout)

		resultCh := make(chan imageJobResult, 1)
		go func() {
			defer cancel()
			resultCh <- computeImageJob(detachedCtx, cfg, service, renderer, input, target, imageCacheKey, imageOptions)
		}()

		select {
		case <-r.Context().Done():
			return
		case result := <-resultCh:
			if result.err != nil {
				if !input.Refresh {
					cachedImage, cacheErr := service.GetImageCache(context.Background(), imageCacheKey)
					if cacheErr == nil && cachedImage != nil {
						w.Header().Set("Content-Type", "image/png")
						w.Header().Set("X-Image-Cache-Status", "stale")
						_, _ = w.Write(cachedImage.Content)
						return
					}
				}

				switch {
				case errors.Is(result.err, domain.ErrInvalidTarget),
					errors.Is(result.err, domain.ErrTargetNotFound),
					errors.Is(result.err, domain.ErrRateLimited),
					errors.Is(result.err, domain.ErrUpstream):
					writeDomainError(w, result.err)
				default:
					writeError(w, nethttp.StatusBadRequest, "image_render_error", result.err.Error())
				}
				return
			}

			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("X-Cache-Status", result.response.Cache.Status)
			w.Header().Set("X-Image-Cache-Status", "miss")
			w.Header().Set("X-GitHub-Requests", strconv.Itoa(result.response.GitHubRequestCount))
			_, _ = w.Write(result.pngBytes)
		}
	}
}

func writeJSON(w nethttp.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeDomainError(w nethttp.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidTarget):
		writeError(w, nethttp.StatusBadRequest, "invalid_target", err.Error())
	case errors.Is(err, domain.ErrTargetNotFound):
		writeError(w, nethttp.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, domain.ErrRateLimited):
		writeError(w, nethttp.StatusTooManyRequests, "rate_limited", err.Error())
	case errors.Is(err, domain.ErrUpstream):
		writeError(w, nethttp.StatusBadGateway, "upstream_error", err.Error())
	default:
		writeError(w, nethttp.StatusInternalServerError, "internal_error", err.Error())
	}
}

func writeError(w nethttp.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func optionalInt(r *nethttp.Request, key string) (int, error) {
	value, _, err := optionalIntWithPresence(r, key)
	return value, err
}

func optionalBool(r *nethttp.Request, key string) (bool, bool, error) {
	rawValues, ok := r.URL.Query()[key]
	if !ok || len(rawValues) == 0 {
		return false, false, nil
	}
	raw := strings.TrimSpace(rawValues[0])
	if raw == "" {
		return false, true, nil
	}
	value, err := strconv.ParseBool(raw)
	return value, true, err
}

func optionalIntWithPresence(r *nethttp.Request, key string) (int, bool, error) {
	rawValues, ok := r.URL.Query()[key]
	if !ok || len(rawValues) == 0 {
		return 0, false, nil
	}
	raw := strings.TrimSpace(rawValues[0])
	if raw == "" {
		return 0, true, nil
	}
	value, err := strconv.Atoi(raw)
	return value, true, err
}

func buildImageCacheKey(target domain.Target, options imagegrid.Options, includeForks bool, includeBots bool) (string, error) {
	normalized, err := imagegrid.NormalizeOptionsForCache(options)
	if err != nil {
		return "", err
	}
	return target.NormalizedTarget + "|image|per_row=" + strconv.Itoa(normalized.PerRow) +
		"|width=" + strconv.Itoa(normalized.Width) +
		"|shape=" + string(normalized.Shape) +
		"|limit=" + strconv.Itoa(normalized.Limit) +
		"|padding=" + strconv.Itoa(normalized.Padding) +
		"|space=" + strconv.Itoa(normalized.Space) +
		"|include_forks=" + strconv.FormatBool(includeForks) +
		"|include_bots=" + strconv.FormatBool(includeBots), nil
}

func ttlForTargetMode(cfg config.Config, mode string) time.Duration {
	if mode == domain.ModeSingleRepo {
		return cfg.CacheTTLSingleRepo
	}
	return cfg.CacheTTLUserOrg
}

func computeImageJob(
	ctx context.Context,
	cfg config.Config,
	service *usecase.Service,
	renderer *imagegrid.Renderer,
	input usecase.GetContributionsInput,
	target domain.Target,
	imageCacheKey string,
	imageOptions imagegrid.Options,
) imageJobResult {
	response, err := service.GetContributions(ctx, input)
	if err != nil {
		return imageJobResult{err: err}
	}

	img, err := renderer.Render(ctx, response.Summary, imageOptions)
	if err != nil {
		return imageJobResult{err: err}
	}

	pngBytes, err := imagegrid.EncodePNGBytes(img)
	if err != nil {
		return imageJobResult{err: err}
	}

	if err := service.SaveImageCache(ctx, domain.ImageCacheRecord{
		CacheKey:    imageCacheKey,
		Target:      target,
		ImagePNG:    pngBytes,
		CacheStatus: "miss",
		ExpiresAt:   time.Now().UTC().Add(ttlForTargetMode(cfg, target.Mode)),
	}); err != nil {
		return imageJobResult{err: err}
	}

	return imageJobResult{
		response: response,
		pngBytes: pngBytes,
	}
}
