package domain

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
)

// Handler is the HTTP layer for custom domains.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes registers this feature's endpoints on r. The orchestrator mounts it at
// /workspaces/{workspaceId}/domains behind the workspace membership middleware.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.add)
	r.Get("/{domainId}", h.get)
	r.Post("/{domainId}/verify", h.verify)
	r.Post("/{domainId}/default", h.setDefault)
	r.Delete("/{domainId}", h.delete)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())

	domains, err := h.svc.List(r.Context(), m.WorkspaceID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	// No DNS instructions here: the list would otherwise hand out every
	// workspace verification token on a page most people only skim.
	items := make([]domainResponse, 0, len(domains))
	for _, d := range domains {
		items = append(items, newDomainResponse(d))
	}
	httpx.List(w, items, httpx.Meta{})
}

func (h *Handler) add(w http.ResponseWriter, r *http.Request) {
	m, err := requireManage(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var req addRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	d, err := h.svc.Add(r.Context(), m.WorkspaceID, req.Hostname)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	// Returned with instructions: the user's next action is to create these
	// records, and making them fetch the domain again to see them is pointless.
	httpx.Data(w, http.StatusCreated, newDomainResponse(d).withInstructions(h.svc.Instructions(d)))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())

	id, err := domainID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	d, err := h.svc.Get(r.Context(), m.WorkspaceID, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, newDomainResponse(d).withInstructions(h.svc.Instructions(d)))
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	m, err := requireManage(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	id, err := domainID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	d, err := h.svc.Verify(r.Context(), m.WorkspaceID, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	// 200 even when verification failed: the check ran, and the result is in
	// status and verification_error. Instructions come back so the UI can show
	// the expected records next to what was actually found.
	httpx.Data(w, http.StatusOK, newDomainResponse(d).withInstructions(h.svc.Instructions(d)))
}

func (h *Handler) setDefault(w http.ResponseWriter, r *http.Request) {
	m, err := requireManage(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	id, err := domainID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	d, err := h.svc.SetDefault(r.Context(), m.WorkspaceID, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, newDomainResponse(d).withInstructions(h.svc.Instructions(d)))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	m, err := requireManage(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	id, err := domainID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if err := h.svc.Delete(r.Context(), m.WorkspaceID, id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// requireManage resolves the membership and enforces the domain permission.
// DNS and TLS changes are admin-and-above because a wrong hostname can take
// live short links offline.
func requireManage(r *http.Request) (authctx.Membership, error) {
	m := authctx.MustMembership(r.Context())
	if !m.Role.CanManageDomains() {
		return m, httpx.ErrForbidden
	}
	return m, nil
}

func domainID(r *http.Request) (uuid.UUID, error) {
	return httpx.UUIDParam(chi.URLParam(r, "domainId"))
}
