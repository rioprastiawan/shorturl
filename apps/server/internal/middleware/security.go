package middleware

import "net/http"

// SecurityHeaders sets conservative defaults on every response. The dashboard
// is served by Nuxt behind the same proxy, so this only covers responses the Go
// server produces itself: API JSON, health checks, and redirects.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
