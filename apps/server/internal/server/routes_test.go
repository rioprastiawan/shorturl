package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rioprastiawan/shorturl/apps/server/internal/middleware"
)

// TestRateLimitCredentials pins the rule that only credential submissions
// consume the strict per-IP budget.
//
// The regression this guards against is real and was found end-to-end: with
// the limiter on the whole /auth group, the dashboard's per-page-load calls to
// /auth/me and /setup/status exhausted a 10-per-minute allowance after a few
// navigations and signed the user out.
func TestRateLimitCredentials(t *testing.T) {
	tests := map[string]struct {
		method  string
		path    string
		limited bool
	}{
		"login is limited":           {http.MethodPost, "/api/v1/auth/login", true},
		"register is limited":        {http.MethodPost, "/api/v1/auth/register", true},
		"setup is limited":           {http.MethodPost, "/api/v1/setup", true},
		"session check is not":       {http.MethodGet, "/api/v1/auth/me", false},
		"setup status is not":        {http.MethodGet, "/api/v1/setup/status", false},
		"logout is not":              {http.MethodPost, "/api/v1/auth/logout", false},
		"GET on a login path is not": {http.MethodGet, "/api/v1/auth/login", false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Budget of one, so a second request reveals whether the limiter
			// was engaged at all.
			rl := middleware.NewRateLimiter(1)
			handler := rateLimitCredentials(rl)(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
			))

			var lastCode int
			for range 3 {
				req := httptest.NewRequest(tc.method, tc.path, nil)
				req.RemoteAddr = "203.0.113.5:1234"
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				lastCode = rec.Code
			}

			gotLimited := lastCode == http.StatusTooManyRequests
			if gotLimited != tc.limited {
				t.Errorf("after 3 requests to %s %s: limited = %v, want %v (status %d)",
					tc.method, tc.path, gotLimited, tc.limited, lastCode)
			}
		})
	}
}

func TestIsAppHost(t *testing.T) {
	s := &Server{}
	s.cfg.AppDomain = "short.example.com"

	appHosts := []string{
		"short.example.com",
		"SHORT.EXAMPLE.COM",
		"short.example.com:443",
		"localhost",
		"localhost:3000",
		"127.0.0.1:8080",
		"app.localhost",
	}
	for _, host := range appHosts {
		if !s.isAppHost(host) {
			t.Errorf("isAppHost(%q) = false, want true", host)
		}
	}

	// A verified custom domain points at this same container. The dashboard
	// API must not answer there.
	shortLinkHosts := []string{"go.example.com", "link.company.id", "", "evil.test"}
	for _, host := range shortLinkHosts {
		if s.isAppHost(host) {
			t.Errorf("isAppHost(%q) = true, want false", host)
		}
	}
}
