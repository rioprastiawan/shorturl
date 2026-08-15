package apikey

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
)

// Handler is the dashboard-facing key management surface.
type Handler struct {
	svc *Service
}

// NewHandler builds the handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes registers key management. The orchestrator mounts it at
// /workspaces/{workspaceId}/api-keys behind the session and workspace
// middleware — this is not part of the machine-to-machine API, because a key
// must never be able to mint another key.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Delete("/{apiKeyId}", h.revoke)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	m, err := requireManage(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 || parsed > 100 {
			httpx.Error(w, r, httpx.BadRequest("limit must be between 1 and 100"))
			return
		}
		limit = parsed
	}
	keys, nextCursor, err := h.svc.List(r.Context(), m.WorkspaceID, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items := make([]DTO, 0, len(keys))
	for _, k := range keys {
		items = append(items, ToDTO(k))
	}
	httpx.List(w, items, httpx.Meta{NextCursor: nextCursor})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	m, err := requireManage(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var req createRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	createdBy := m.UserID
	key, plaintext, err := h.svc.Create(r.Context(), m.WorkspaceID, &createdBy,
		req.Name, req.Scopes, req.ExpiresAt, req.Test)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusCreated, newCreatedDTO(key, plaintext))
}

func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) {
	m, err := requireManage(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	id, err := httpx.UUIDParam(chi.URLParam(r, "apiKeyId"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if err := h.svc.Revoke(r.Context(), m.WorkspaceID, id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// requireManage gates every route on CanManageAPIKeys: a key grants
// workspace-wide access that outlives any session, so issuing one is an
// admin-and-above act.
func requireManage(r *http.Request) (authctx.Membership, error) {
	m := authctx.MustMembership(r.Context())
	if !m.Role.CanManageAPIKeys() {
		return m, httpx.ErrForbidden
	}
	return m, nil
}
