package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLoggerIncludesImageContributionDetails(t *testing.T) {
	record := logRequest(t, "/v1/contributions/image?target=acme/widget&refresh=true&include_forks=true&include_bots=false&per_row=8&width=800&shape=square&limit=24&padding=4&space=2")

	assertStringField(t, record, "output_format", "image")
	assertBoolField(t, record, "image_request", true)
	assertBoolField(t, record, "json_request", false)
	assertStringField(t, record, "target", "acme/widget")
	assertStringField(t, record, "normalized_target", "acme/widget")
	assertStringField(t, record, "target_mode", "single_repo")
	assertStringField(t, record, "owner", "acme")
	assertStringField(t, record, "owner_or_org", "acme")
	assertStringField(t, record, "repo", "widget")

	assertBoolField(t, record, "refresh", true)
	assertBoolField(t, record, "include_forks", true)
	assertBoolField(t, record, "include_bots", false)
	assertIntField(t, record, "per_row", 8)
	assertIntField(t, record, "width", 800)
	assertStringField(t, record, "shape", "square")
	assertIntField(t, record, "limit", 24)
	assertIntField(t, record, "padding", 4)
	assertIntField(t, record, "space", 2)
	assertMissingField(t, record, "request_query")
}

func TestRequestLoggerIncludesJSONContributionDetails(t *testing.T) {
	record := logRequest(t, "/v1/contributions?target=acme&summary=false&include_bots=true")

	assertStringField(t, record, "output_format", "json")
	assertBoolField(t, record, "image_request", false)
	assertBoolField(t, record, "json_request", true)
	assertStringField(t, record, "target", "acme")
	assertStringField(t, record, "normalized_target", "acme")
	assertStringField(t, record, "target_mode", "user_or_org")
	assertStringField(t, record, "owner", "acme")
	assertStringField(t, record, "owner_or_org", "acme")
	assertStringField(t, record, "repo", "")

	assertBoolField(t, record, "summary", false)
	assertBoolField(t, record, "include_bots", true)
	assertMissingField(t, record, "request_query")
}

func TestRequestLoggerTextOutputFlattensContributionQuery(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	handler := RequestLogger(logger, NewRealIPResolver())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "/v1/contributions/image?target=yorukot/superfile&refresh=false&include_forks=false&include_bots=false&per_row=12&width=1920&shape=square&limit=144&padding=0&space=12", nil)

	handler.ServeHTTP(httptest.NewRecorder(), request)

	output := buf.String()
	for _, want := range []string{
		"include_bots=false",
		"include_forks=false",
		"refresh=false",
		"per_row=12",
		"width=1920",
		"shape=square",
		"limit=144",
		"padding=0",
		"space=12",
		"target=yorukot/superfile",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected text log to contain %q, got %q", want, output)
		}
	}
	if strings.Contains(output, "request_query") || strings.Contains(output, "map[") {
		t.Fatalf("expected flattened query fields, got %q", output)
	}
}

func TestRequestLoggerLogsOnlyResolvedRemoteAddr(t *testing.T) {
	headers := http.Header{}
	headers.Set("CF-Connecting-IP", "203.0.113.10")
	headers.Set("True-Client-IP", "203.0.113.11")
	headers.Set("X-Real-IP", "203.0.113.12")
	headers.Set("X-Forwarded-For", "203.0.113.13, 10.0.0.5")
	headers.Set("CF-Ray", "abc123-TPE")
	headers.Set("CF-IPCountry", "TW")

	record := logRequestWithResolver(t, "/v1/contributions?target=acme", "10.0.0.5:443", headers, NewRealIPResolver())

	assertStringField(t, record, "remote_addr", "203.0.113.10")
	assertMissingField(t, record, "real_ip")
	assertMissingField(t, record, "real_ip_source")
	assertMissingField(t, record, "socket_remote_addr")
	assertMissingField(t, record, "cf_connecting_ip")
	assertMissingField(t, record, "x_forwarded_for")
}

func logRequest(t *testing.T, target string) map[string]any {
	t.Helper()

	return logRequestWithResolver(t, target, "192.0.2.1:1234", nil, NewRealIPResolver())
}

func logRequestWithResolver(t *testing.T, target string, remoteAddr string, headers http.Header, resolver RealIPResolver) map[string]any {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	handler := RequestLogger(logger, resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.RemoteAddr = remoteAddr
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", recorder.Code)
	}

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("decode log record: %v", err)
	}
	return record
}

func assertStringField(t *testing.T, record map[string]any, key string, want string) {
	t.Helper()

	got, ok := record[key].(string)
	if !ok {
		t.Fatalf("expected %q to be a string, got %#v", key, record[key])
	}
	if got != want {
		t.Fatalf("expected %q to be %q, got %q", key, want, got)
	}
}

func assertBoolField(t *testing.T, record map[string]any, key string, want bool) {
	t.Helper()

	got, ok := record[key].(bool)
	if !ok {
		t.Fatalf("expected %q to be a bool, got %#v", key, record[key])
	}
	if got != want {
		t.Fatalf("expected %q to be %t, got %t", key, want, got)
	}
}

func assertIntField(t *testing.T, record map[string]any, key string, want int) {
	t.Helper()

	got, ok := record[key].(float64)
	if !ok {
		t.Fatalf("expected %q to be a number, got %#v", key, record[key])
	}
	if got != float64(want) {
		t.Fatalf("expected %q to be %d, got %v", key, want, got)
	}
}

func assertMissingField(t *testing.T, record map[string]any, key string) {
	t.Helper()

	if _, ok := record[key]; ok {
		t.Fatalf("expected %q to be omitted, got %#v", key, record[key])
	}
}
