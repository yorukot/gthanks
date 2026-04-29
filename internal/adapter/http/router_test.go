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

	"gthanks/internal/adapter/imagegrid"
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
		"GThanks",
		"/image",
		"/json",
		"include_bots",
		`id="include-bots" type="checkbox" checked`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected index HTML to contain %q", want)
		}
	}
	if strings.Contains(body, "/healthz") || strings.Contains(body, ">Health<") {
		t.Fatal("expected index HTML not to expose the health endpoint")
	}
	if strings.Contains(body, "/v1/contributions") {
		t.Fatal("expected index HTML to use simplified endpoints only")
	}
	imageIndex := strings.Index(body, `value="image"`)
	jsonIndex := strings.Index(body, `value="json"`)
	if imageIndex == -1 || jsonIndex == -1 || imageIndex > jsonIndex {
		t.Fatal("expected image endpoint option to appear before JSON")
	}
}

func TestSimplifiedContributionRoutes(t *testing.T) {
	router := testRouter()

	for _, path := range []string{"/json", "/image"} {
		request := httptest.NewRequest(nethttp.MethodGet, path, nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code == nethttp.StatusNotFound {
			t.Fatalf("expected %s route to exist", path)
		}
	}

	for _, path := range []string{"/v1/contributions", "/v1/contributions/image"} {
		request := httptest.NewRequest(nethttp.MethodGet, path, nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code != nethttp.StatusNotFound {
			t.Fatalf("expected legacy route %s to return 404, got %d", path, recorder.Code)
		}
	}
}

func TestImageCacheKeyIncludesBotOption(t *testing.T) {
	target := domain.Target{NormalizedTarget: "yorukot/repo"}
	withoutBots, err := buildImageCacheKey(target, imagegridOptionsForTest(), false, false)
	if err != nil {
		t.Fatalf("build image cache key without bots: %v", err)
	}
	withBots, err := buildImageCacheKey(target, imagegridOptionsForTest(), false, true)
	if err != nil {
		t.Fatalf("build image cache key with bots: %v", err)
	}

	if withoutBots == withBots {
		t.Fatal("expected include_bots to change image cache key")
	}
	if !strings.Contains(withoutBots, "include_bots=false") {
		t.Fatalf("expected bot-filtered image cache key, got %q", withoutBots)
	}
	if !strings.Contains(withBots, "include_bots=true") {
		t.Fatalf("expected bot-included image cache key, got %q", withBots)
	}
}

func imagegridOptionsForTest() imagegrid.Options {
	return imagegrid.Options{
		PerRow:   12,
		Width:    1920,
		Shape:    imagegrid.ShapeCircle,
		Limit:    144,
		Padding:  0,
		Space:    12,
		SpaceSet: true,
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
