package apikey

import (
	"net/http"
	"strings"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
)

// RequireAPIKey authenticates the bearer token and attaches the machine caller
// to the context. Everything below it may call authctx.MustAPIKey.
func RequireAPIKey(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := BearerToken(r)
			if !ok {
				httpx.Error(w, r, httpx.ErrUnauthorized)
				return
			}

			principal, err := svc.Authenticate(r.Context(), token)
			if err != nil {
				httpx.Error(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(authctx.WithAPIKey(r.Context(), principal)))
		})
	}
}

// RequireScope rejects a key that was not granted the scope this route needs.
//
// The distinct 403 code lets an integration tell "your key is fine but too
// narrow" from "your key is bad", which are different fixes.
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := authctx.APIKey(r.Context())
			if !ok {
				httpx.Error(w, r, httpx.ErrUnauthorized)
				return
			}
			if !principal.HasScope(scope) {
				httpx.Error(w, r, insufficientScope(scope))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func insufficientScope(scope string) *httpx.APIError {
	return &httpx.APIError{
		Status:  http.StatusForbidden,
		Code:    "insufficient_scope",
		Message: "This API key does not have the " + scope + " scope",
	}
}

// BearerToken reads the token out of the Authorization header. The scheme is
// matched case-insensitively because RFC 7235 says it is.
func BearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}
