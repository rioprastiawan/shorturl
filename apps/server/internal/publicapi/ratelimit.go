package publicapi

import (
	"net/http"
	"strconv"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/middleware"
)

// retryAfterSeconds matches the limiter's per-minute window.
const retryAfterSeconds = "60"

// RateLimitByAPIKey limits the public API per key rather than per IP
// (plan §14.10).
//
// Keying on the IP would be wrong in both directions: a whole office behind
// one NAT shares an address, and a caller running on several hosts would get
// several times its allowance. The key is the thing the limit is sold against.
//
// Must be mounted after apikey.RequireAPIKey. If no principal is present the
// client address is used, so an unauthenticated flood still costs something.
func RateLimitByAPIKey(rl *middleware.RateLimiter, perMinute int) func(http.Handler) http.Handler {
	limit := strconv.Itoa(perMinute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Limit", limit)

			key := "ip:" + middleware.ClientIP(r)
			if principal, ok := authctx.APIKey(r.Context()); ok {
				key = "key:" + principal.ID.String()
			}

			if !rl.Allow(key) {
				w.Header().Set("Retry-After", retryAfterSeconds)
				httpx.Error(w, r, httpx.ErrRateLimited)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
