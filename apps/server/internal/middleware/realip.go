package middleware

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type ctxKey int

const realIPKey ctxKey = iota

// RealIP resolves the client address, trusting X-Forwarded-For only when the
// immediate peer is one of the configured proxies.
//
// chi's stock RealIP trusts the header from anyone, which lets any caller
// choose the address that rate limiting and analytics key on. Here the header
// is walked from right to left, discarding hops that are themselves trusted
// proxies, and the first untrusted address is the real client.
func RealIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := resolveIP(r, trusted)
			ctx := context.WithValue(r.Context(), realIPKey, ip)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClientIP returns the address resolved by RealIP, falling back to the raw
// peer address when the middleware is not installed.
func ClientIP(r *http.Request) string {
	if ip, ok := r.Context().Value(realIPKey).(string); ok && ip != "" {
		return ip
	}
	return peerIP(r)
}

func resolveIP(r *http.Request, trusted []netip.Prefix) string {
	peer := peerIP(r)
	if !isTrusted(peer, trusted) {
		// Direct connection from an untrusted source: believe only the socket.
		return peer
	}

	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded == "" {
		if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); real != "" && parseable(real) {
			return real
		}
		return peer
	}

	hops := strings.Split(forwarded, ",")
	for i := len(hops) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(hops[i])
		if !parseable(candidate) {
			continue
		}
		if isTrusted(candidate, trusted) {
			continue // another proxy of ours; keep walking left
		}
		return candidate
	}

	// Every hop was a trusted proxy, which means the request originated inside
	// the trusted network.
	return peer
}

func peerIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func parseable(s string) bool {
	_, err := netip.ParseAddr(s)
	return err == nil
}

func isTrusted(ip string, trusted []netip.Prefix) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
