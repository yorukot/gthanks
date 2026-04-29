package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type RealIPResolver struct{}

func NewRealIPResolver() RealIPResolver {
	return RealIPResolver{}
}

func (RealIPResolver) Resolve(r *http.Request) string {
	for _, value := range []string{
		r.Header.Get("CF-Connecting-IP"),
		r.Header.Get("True-Client-IP"),
		r.Header.Get("X-Real-IP"),
	} {
		if ip, ok := parseHeaderIP(value); ok {
			return ip.String()
		}
	}

	if ip, ok := firstValidForwardedForIP(r.Header.Get("X-Forwarded-For")); ok {
		return ip.String()
	}

	if ip, ok := parseRemoteAddrIP(r.RemoteAddr); ok {
		return ip.String()
	}
	return r.RemoteAddr
}

func parseHeaderIP(value string) (netip.Addr, bool) {
	value = strings.Trim(strings.TrimSpace(value), "[]")
	if value == "" {
		return netip.Addr{}, false
	}

	ip, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

func parseRemoteAddrIP(remoteAddr string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = strings.Trim(strings.TrimSpace(remoteAddr), "[]")
	}
	return parseHeaderIP(host)
}

func firstValidForwardedForIP(value string) (netip.Addr, bool) {
	for _, part := range strings.Split(value, ",") {
		if ip, ok := parseHeaderIP(part); ok {
			return ip, true
		}
	}
	return netip.Addr{}, false
}
