package middleware

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gthanks/internal/domain"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

const (
	jsonContributionsPath  = "/v1/contributions"
	imageContributionsPath = "/v1/contributions/image"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func RequestLogger(logger *slog.Logger, realIPResolver RealIPResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			realIP := realIPResolver.Resolve(r)

			next.ServeHTTP(recorder, r)

			attrs := []any{
				"request_id", chimiddleware.GetReqID(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"remote_addr", realIP,
			}
			attrs = append(attrs, contributionRequestAttrs(r)...)

			logger.Info("http_request", attrs...)
		})
	}
}

func contributionRequestAttrs(r *http.Request) []any {
	outputFormat := contributionOutputFormat(r.URL.Path)
	if outputFormat == "" {
		return nil
	}

	query := r.URL.Query()
	attrs := []any{
		"output_format", outputFormat,
		"image_request", outputFormat == "image",
		"json_request", outputFormat == "json",
	}
	attrs = appendContributionQueryAttrs(attrs, query)

	rawTarget := query.Get("target")
	if strings.TrimSpace(rawTarget) == "" {
		return attrs
	}
	attrs = append(attrs, "target", rawTarget)

	target, err := domain.ParseTarget(rawTarget)
	if err != nil {
		return append(attrs, "target_parse_error", err.Error())
	}

	return append(attrs,
		"normalized_target", target.NormalizedTarget,
		"target_mode", target.Mode,
		"owner", target.Owner,
		"owner_or_org", target.Owner,
		"repo", target.Repo,
	)
}

func contributionOutputFormat(path string) string {
	switch path {
	case imageContributionsPath:
		return "image"
	case jsonContributionsPath:
		return "json"
	default:
		return ""
	}
}

func appendContributionQueryAttrs(attrs []any, query url.Values) []any {
	for _, key := range []string{"refresh", "summary", "include_forks", "include_bots"} {
		attrs = appendQueryBoolAttr(attrs, query, key)
	}
	for _, key := range []string{"per_row", "width", "limit", "padding", "space"} {
		attrs = appendQueryIntAttr(attrs, query, key)
	}
	return appendQueryStringAttr(attrs, query, "shape")
}

func appendQueryBoolAttr(attrs []any, query url.Values, key string) []any {
	raw, ok := firstQueryValue(query, key)
	if !ok {
		return attrs
	}
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return append(attrs, key, raw)
	}
	return append(attrs, key, value)
}

func appendQueryIntAttr(attrs []any, query url.Values, key string) []any {
	raw, ok := firstQueryValue(query, key)
	if !ok {
		return attrs
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return append(attrs, key, raw)
	}
	return append(attrs, key, value)
}

func appendQueryStringAttr(attrs []any, query url.Values, key string) []any {
	raw, ok := firstQueryValue(query, key)
	if !ok {
		return attrs
	}
	return append(attrs, key, raw)
}

func firstQueryValue(query url.Values, key string) (string, bool) {
	values, ok := query[key]
	if !ok || len(values) == 0 {
		return "", false
	}
	return values[0], true
}
