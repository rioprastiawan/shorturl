package auth

import (
	"net/http"
	"time"

	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
)

// SetSessionCookie writes the session cookie.
//
// SameSite=Lax rather than Strict: the dashboard is reached by following links
// from outside the app, and a Strict cookie would leave those navigations
// logged out. It still blocks the cross-site POSTs that CSRF depends on.
//
// Exported as a function taking config so the setup wizard can log its new
// admin in without depending on an auth.Service.
func SetSessionCookie(w http.ResponseWriter, cfg config.Config, token string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cfg.SessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie expires the session cookie. The attributes must match the
// ones it was set with, or the browser keeps the original cookie alongside it.
func ClearSessionCookie(w http.ResponseWriter, cfg config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.SessionCookie,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ReadCookie returns the plaintext session token from the request, or an empty
// string when the cookie is absent.
func (s *Service) ReadCookie(r *http.Request) string {
	c, err := r.Cookie(s.cfg.SessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// SetCookie writes the session cookie using the service's configuration.
func (s *Service) SetCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	SetSessionCookie(w, s.cfg, token, expiresAt)
}

// ClearCookie expires the session cookie using the service's configuration.
func (s *Service) ClearCookie(w http.ResponseWriter) {
	ClearSessionCookie(w, s.cfg)
}
