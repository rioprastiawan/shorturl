package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/rioprastiawan/shorturl/apps/server/internal/analytics"
	"github.com/rioprastiawan/shorturl/apps/server/internal/apikey"
	"github.com/rioprastiawan/shorturl/apps/server/internal/auth"
	"github.com/rioprastiawan/shorturl/apps/server/internal/domain"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/link"
	"github.com/rioprastiawan/shorturl/apps/server/internal/middleware"
	"github.com/rioprastiawan/shorturl/apps/server/internal/redirect"
	"github.com/rioprastiawan/shorturl/apps/server/internal/setup"
	"github.com/rioprastiawan/shorturl/apps/server/internal/urlx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/workspace"
)

// routes builds the full route tree.
//
// One process serves two very different surfaces: the dashboard API on the
// application domain, and short-link redirects on every custom domain. They
// are separated by Host, not by port, so a single container and a single
// certificate resolver cover both.
func (s *Server) routes(version string) http.Handler {
	r := chi.NewRouter()

	// Order follows plan §36. Real-IP comes before logging and rate limiting
	// because both key on the address it resolves.
	r.Use(chimw.RequestID)
	r.Use(middleware.RealIP(s.cfg.TrustedProxies))
	r.Use(chimw.Recoverer)
	r.Use(middleware.RequestLogger(s.logger))
	r.Use(middleware.SecurityHeaders)
	r.Use(chimw.Timeout(30 * time.Second))

	// Probes answer on any Host: Docker's healthcheck connects to 127.0.0.1
	// and would never match the configured application domain.
	r.Get("/health", healthHandler(version))
	r.Get("/ready", s.readyHandler())

	if s.app != nil {
		r.Route("/api/v1", func(r chi.Router) {
			// The API is reachable only on the application domain. A verified
			// custom domain points at this same container, and exposing the
			// dashboard API there would put an authentication surface on a
			// hostname the operator thinks only serves redirects.
			r.Use(s.requireAppHost)
			s.mountAPI(r)
		})

		// Anything else is a short link. On a custom domain that is every
		// path; on the application domain the reserved-slug list keeps links
		// from shadowing /api, /login, and friends (plan §8, §40).
		redirectHandler := redirect.NewHandler(s.app.Link, s.app.Producer, s.cfg, s.logger)
		redirectLimiter := middleware.NewRateLimiter(s.cfg.RateLimitRedirect)
		r.NotFound(middleware.RateLimit(redirectLimiter, middleware.ByIP)(redirectHandler).ServeHTTP)
		r.MethodNotAllowed(middleware.RateLimit(redirectLimiter, middleware.ByIP)(redirectHandler).ServeHTTP)
	}

	return r
}

// mountAPI registers every /api/v1 endpoint.
func (s *Server) mountAPI(r chi.Router) {
	app := s.app

	authHandler := auth.NewHandler(app.Auth)
	setupHandler := setup.NewHandler(app.Setup)
	wsHandler := workspace.NewHandler(app.Workspace)
	domainHandler := domain.NewHandler(app.Domain)
	linkHandler := link.NewHandler(app.Link)
	analyticsHandler := analytics.NewHandler(app.Analytics)
	apiKeyHandler := apikey.NewHandler(app.APIKey)
	publicHandler := app.PublicAPIHandler()

	authLimiter := middleware.NewRateLimiter(s.cfg.RateLimitAuth)
	apiLimiter := middleware.NewRateLimiter(s.cfg.RateLimitAPI)

	// The strict per-IP limit belongs on the endpoints that accept credentials
	// and nothing else. /auth/me and /setup/status are polled on every page
	// load by the dashboard, so including them would spend a 10-per-minute
	// budget on ordinary navigation and sign the user out mid-session.
	guardCredentials := rateLimitCredentials(authLimiter)

	// First-run wizard. Unauthenticated by necessity — there is no account to
	// authenticate with yet.
	r.Group(func(r chi.Router) {
		r.Use(guardCredentials)
		setupHandler.Routes(r)
	})

	// Session authentication.
	r.Route("/auth", func(r chi.Router) {
		r.Use(guardCredentials)
		authHandler.Mount(r)
	})

	// Dashboard, session-authenticated and workspace-scoped.
	r.Route("/workspaces", func(r chi.Router) {
		r.Use(auth.RequireAuth(app.Auth))
		wsHandler.Routes(r)

		r.Route("/{workspaceId}", func(r chi.Router) {
			// Must be registered inside this closure: chi only populates URL
			// parameters once the segment matches, so a parent-level Use would
			// read an empty workspaceId and authorize nothing.
			r.Use(workspace.RequireWorkspace(app.Workspace))

			wsHandler.WorkspaceRoutes(r)
			r.Route("/domains", domainHandler.Routes)
			r.Route("/analytics", analyticsHandler.Routes)
			r.Route("/api-keys", apiKeyHandler.Routes)

			r.Route("/links", func(r chi.Router) {
				linkHandler.Routes(r)
				r.Get("/{linkId}/analytics", analyticsHandler.LinkAnalytics)
			})
		})
	})

	// Machine-to-machine API. The workspace comes from the key, never from the
	// request, and the limit is per key rather than per IP so one noisy
	// integration cannot exhaust another's allowance from the same NAT.
	r.Route("/links", func(r chi.Router) {
		r.Use(apikey.RequireAPIKey(app.APIKey))
		r.Use(publicapiRateLimit(apiLimiter, s.cfg.RateLimitAPI))
		publicHandler.Routes(r)
	})
}

// credentialPaths are the endpoints that accept a password. They are the ones
// worth a strict limit: everything else under /auth and /setup is a read that
// the dashboard performs routinely.
var credentialPaths = map[string]bool{
	"/api/v1/auth/login":    true,
	"/api/v1/auth/register": true,
	"/api/v1/setup":         true,
}

// rateLimitCredentials applies the limiter only to credential submissions.
//
// Scoping by path rather than by route group keeps the limit meaningful: a
// budget spent on session checks protects nothing, and exhausting it logs
// legitimate users out. Only POST counts, so a stray GET cannot consume it.
func rateLimitCredentials(rl *middleware.RateLimiter) func(http.Handler) http.Handler {
	limited := middleware.RateLimit(rl, middleware.ByIP)
	return func(next http.Handler) http.Handler {
		guarded := limited(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && credentialPaths[r.URL.Path] {
				guarded.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requireAppHost rejects API traffic arriving on a custom short-link domain.
func (s *Server) requireAppHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.isAppHost(r.Host) {
			next.ServeHTTP(w, r)
			return
		}
		// 404 rather than 403: on a short-link domain, /api/v1/... is simply
		// not a route that exists, and saying so reveals nothing.
		httpx.Error(w, r, httpx.ErrNotFound)
	})
}

func (s *Server) isAppHost(host string) bool {
	normalized := urlx.NormalizeHostname(host)
	if normalized == "" {
		return false
	}
	if normalized == s.cfg.AppDomain {
		return true
	}
	// Local development and container health checks never carry the real
	// application domain.
	switch normalized {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return true
	}
	return strings.HasSuffix(normalized, ".localhost")
}
