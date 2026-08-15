package auth

import (
	"net/http"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
)

// RequireAuth rejects requests without a valid session cookie and attaches the
// user to the request context for the handlers behind it.
func RequireAuth(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := svc.Authenticate(r.Context(), svc.ReadCookie(r))
			if err != nil {
				httpx.Error(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(authctx.WithUser(r.Context(), user)))
		})
	}
}

// OptionalAuth attaches the user when a valid session is present and otherwise
// lets the request through unauthenticated. Used by routes that serve both
// signed-in and anonymous callers, such as the setup status endpoint.
func OptionalAuth(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token := svc.ReadCookie(r); token != "" {
				if user, err := svc.Authenticate(r.Context(), token); err == nil {
					r = r.WithContext(authctx.WithUser(r.Context(), user))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
