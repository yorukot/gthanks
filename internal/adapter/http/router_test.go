package http

import (
	"context"
	"io"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gthanks/internal/config"
	"gthanks/internal/domain"
	"gthanks/internal/usecase"
)

type fakeStore struct{}

func (fakeStore) GetQueryCache(context.Context, string) (*domain.CachedResponse, error) {
	return nil, nil
}

func (fakeStore) SaveQueryCache(context.Context, domain.QueryCacheRecord) error {
	return nil
}

func (fakeStore) GetImageCache(context.Context, string) (*domain.CachedBinary, error) {
	return nil, nil
}

func (fakeStore) SaveImageCache(context.Context, domain.ImageCacheRecord) error {
	return nil
}

func (fakeStore) GetAvatarCache(context.Context, string) (*domain.CachedBinary, error) {
	return nil, nil
}

func (fakeStore) SaveAvatarCache(context.Context, domain.AvatarCacheRecord) error {
	return nil
}

func (fakeStore) SaveRepoSnapshot(context.Context, domain.Target, domain.Repo) error {
	return nil
}

func TestIndexRouteServesURLGenerator(t *testing.T) {
	router := testRouter()
	request := httptest.NewRequest(nethttp.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != nethttp.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected HTML content type, got %q", contentType)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		"gthanks URL generator",
		"/v1/contributions/image",
		"/v1/contributions",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected index HTML to contain %q", want)
		}
	}
	if strings.Contains(body, "/healthz") || strings.Contains(body, ">Health<") {
		t.Fatal("expected index HTML not to expose the health endpoint")
	}
	imageIndex := strings.Index(body, `value="image"`)
	jsonIndex := strings.Index(body, `value="contributions"`)
	if imageIndex == -1 || jsonIndex == -1 || imageIndex > jsonIndex {
		t.Fatal("expected image endpoint option to appear before JSON")
	}
}

func TestHealthRouteStillResponds(t *testing.T) {
	router := testRouter()
	request := httptest.NewRequest(nethttp.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != nethttp.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected JSON content type, got %q", contentType)
	}
}

func testRouter() nethttp.Handler {
	cfg := config.Config{RequestTimeout: time.Second}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := usecase.NewService(cfg, fakeStore{}, nil)
	return NewRouter(cfg, logger, service)
}
