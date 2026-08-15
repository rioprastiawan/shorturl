package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

type readyResponse struct {
	Status   string            `json:"status"`
	Checks   map[string]string `json:"checks"`
	Duration string            `json:"duration_ms,omitempty"`
}

// healthHandler reports liveness of the process only. It performs no
// dependency checks and exposes no configuration, so it is safe to leave
// unauthenticated for Docker and load-balancer probes.
func healthHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Version: version})
	}
}

// readyHandler reports whether the dependencies needed to serve traffic are
// reachable. Kubernetes-style semantics: a 503 means "stop sending me
// requests", not "restart me".
//
// The response names which dependency failed but never why — a connection
// error string can contain the host, port, and user from the DSN.
func (s *Server) readyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.app == nil {
			writeJSON(w, http.StatusOK, readyResponse{
				Status: "ready",
				Checks: map[string]string{},
			})
			return
		}

		// Bounded so a hung dependency fails the probe instead of hanging it.
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		start := time.Now()
		checks := map[string]string{
			"postgres": "ok",
			"redis":    "ok",
		}
		status := http.StatusOK

		if err := s.app.Pool.Ping(ctx); err != nil {
			checks["postgres"] = "unavailable"
			status = http.StatusServiceUnavailable
		}
		if err := s.app.Redis.Ping(ctx); err != nil {
			checks["redis"] = "unavailable"
			status = http.StatusServiceUnavailable
		}

		body := readyResponse{Status: "ready", Checks: checks}
		if status != http.StatusOK {
			body.Status = "not ready"
		}
		body.Duration = time.Since(start).Round(time.Millisecond).String()

		writeJSON(w, status, body)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
