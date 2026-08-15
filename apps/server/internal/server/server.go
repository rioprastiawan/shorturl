// Package server wires the HTTP router and runs the listener.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
	"github.com/rioprastiawan/shorturl/apps/server/internal/middleware"
	"github.com/rioprastiawan/shorturl/apps/server/internal/publicapi"
)

// Server owns the HTTP listener and the route tree.
type Server struct {
	cfg     config.Config
	logger  *slog.Logger
	app     *App
	handler http.Handler
}

// New builds a Server with all routes registered.
//
// app may be nil, which yields a health-only server. That is what the tests
// use, and it keeps the probes working even if dependency wiring is skipped.
func New(cfg config.Config, logger *slog.Logger, version string, app *App) *Server {
	s := &Server{cfg: cfg, logger: logger, app: app}
	s.handler = s.routes(version)
	return s
}

// Handler exposes the route tree so tests can exercise it without a listener.
func (s *Server) Handler() http.Handler { return s.handler }

// Run starts the listener and blocks until ctx is cancelled, then drains
// in-flight requests within the configured shutdown timeout.
func (s *Server) Run(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:              s.cfg.Addr(),
		Handler:           s.handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server listening",
			slog.String("addr", httpServer.Addr),
			slog.String("env", s.cfg.AppEnv),
			slog.String("app_domain", s.cfg.AppDomain),
		)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	s.logger.Info("shutting down", slog.Duration("timeout", s.cfg.ShutdownTimeout))
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.ShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return <-errCh
}

// publicapiRateLimit is a thin alias so routes.go reads consistently.
func publicapiRateLimit(rl *middleware.RateLimiter, perMinute int) func(http.Handler) http.Handler {
	return publicapi.RateLimitByAPIKey(rl, perMinute)
}
