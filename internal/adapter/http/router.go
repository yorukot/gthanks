package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	nethttp "net/http"
	"strconv"
	"time"

	"gthanks/internal/adapter/http/middleware"
	"gthanks/internal/config"
	"gthanks/internal/domain"
	"gthanks/internal/usecase"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func NewRouter(cfg config.Config, logger *slog.Logger, service *usecase.Service) nethttp.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(cfg.RequestTimeout))
	r.Use(middleware.RequestLogger(logger))

	r.Get("/healthz", func(w nethttp.ResponseWriter, _ *nethttp.Request) {
		writeJSON(w, nethttp.StatusOK, map[string]any{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})
	r.Get("/v1/contributions", contributionsHandler(service))

	return r
}

func contributionsHandler(service *usecase.Service) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
		input := usecase.GetContributionsInput{
			Target:  r.URL.Query().Get("target"),
			Refresh: r.URL.Query().Get("refresh") == "true",
			Summary: true,
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
