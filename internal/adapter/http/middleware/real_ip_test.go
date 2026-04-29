package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRealIPResolverPrefersCloudflareHeader(t *testing.T) {
	resolver := NewRealIPResolver()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.5:443"
	request.Header.Set("CF-Connecting-IP", "203.0.113.10")
	request.Header.Set("True-Client-IP", "203.0.113.11")
	request.Header.Set("X-Real-IP", "203.0.113.12")
	request.Header.Set("X-Forwarded-For", "203.0.113.13")

	got := resolver.Resolve(request)

	if got != "203.0.113.10" {
		t.Fatalf("expected CF-Connecting-IP real IP, got %q", got)
	}
}

func TestRealIPResolverUsesFirstValidForwardedForIP(t *testing.T) {
	resolver := NewRealIPResolver()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.5:443"
	request.Header.Set("X-Forwarded-For", "not-an-ip, 203.0.113.20, 203.0.113.21")

	got := resolver.Resolve(request)

	if got != "203.0.113.20" {
		t.Fatalf("expected first valid X-Forwarded-For IP, got %q", got)
	}
}

func TestRealIPResolverTrustsForwardedHeadersFromAnyPeer(t *testing.T) {
	resolver := NewRealIPResolver()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "198.51.100.5:443"
	request.Header.Set("CF-Connecting-IP", "203.0.113.10")
	request.Header.Set("X-Forwarded-For", "203.0.113.20")

	got := resolver.Resolve(request)

	if got != "203.0.113.10" {
		t.Fatalf("expected trusted header IP, got %q", got)
	}
}

func TestRealIPResolverFallsBackOnInvalidHeaders(t *testing.T) {
	resolver := NewRealIPResolver()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.5:443"
	request.Header.Set("CF-Connecting-IP", "invalid")
	request.Header.Set("True-Client-IP", "still-invalid")
	request.Header.Set("X-Real-IP", "also-invalid")
	request.Header.Set("X-Forwarded-For", "not-an-ip")

	got := resolver.Resolve(request)

	if got != "10.0.0.5" {
		t.Fatalf("expected fallback socket peer IP, got %q", got)
	}
}

func TestRealIPResolverSupportsIPv6(t *testing.T) {
	resolver := NewRealIPResolver()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "[fd00::1]:443"
	request.Header.Set("CF-Connecting-IP", "2001:db8::10")

	got := resolver.Resolve(request)

	if got != "2001:db8::10" {
		t.Fatalf("expected IPv6 Cloudflare client IP, got %q", got)
	}
}
