package analytics

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
)

// Handler exposes the reporting endpoints.
type Handler struct {
	svc *Service
}

// NewHandler wires the HTTP layer to the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes registers the workspace-scoped analytics endpoints. Mount at
// /workspaces/{workspaceId}/analytics.
//
// The per-link report is not registered here: it lives under the links tree,
// so the orchestrator mounts LinkAnalytics at
// /workspaces/{workspaceId}/links/{linkId}/analytics.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/overview", h.overview)
	r.Get("/clicks", h.clicks)
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	if err := requireReadAccess(r); err != nil {
		httpx.Error(w, r, err)
		return
	}

	data, err := h.svc.Overview(r.Context(), m.WorkspaceID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, data)
}

func (h *Handler) clicks(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	if err := requireReadAccess(r); err != nil {
		httpx.Error(w, r, err)
		return
	}

	rng, err := rangeFromRequest(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	data, err := h.svc.Clicks(r.Context(), m.WorkspaceID, rng)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, data)
}

// LinkAnalytics serves the per-link report. It is exported because it is
// mounted under the links route tree rather than under Routes above.
func (h *Handler) LinkAnalytics(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	if err := requireReadAccess(r); err != nil {
		httpx.Error(w, r, err)
		return
	}

	linkID, err := httpx.UUIDParam(chi.URLParam(r, "linkId"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	rng, err := rangeFromRequest(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	data, err := h.svc.LinkAnalytics(r.Context(), m.WorkspaceID, linkID, rng)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, data)
}

func rangeFromRequest(r *http.Request) (Range, error) {
	query := r.URL.Query()
	return ParseRange(query.Get("range"), query.Get("from"), query.Get("to"), time.Now())
}

// requireReadAccess enforces the analytics scope for machine callers. Every
// workspace member may read analytics, so a session-authenticated request
// needs no further check beyond the membership the middleware resolved.
func requireReadAccess(r *http.Request) error {
	key, ok := authctx.APIKey(r.Context())
	if !ok {
		return nil
	}
	if !key.HasScope(authctx.ScopeAnalyticsRead) {
		return httpx.Forbiddenf("This API key does not have the %s scope", authctx.ScopeAnalyticsRead)
	}
	return nil
}
