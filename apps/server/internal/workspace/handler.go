package workspace

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/validate"
)

// Handler is the HTTP layer over Service.
type Handler struct {
	svc *Service
}

// NewHandler builds the workspace handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes registers the collection endpoints, which need an authenticated user
// but no workspace.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
}

// WorkspaceRoutes registers the endpoints for a single workspace, relative to
// a /{workspaceId} route. The caller must have applied RequireWorkspace to
// that route, because every handler here reads authctx.MustMembership.
func (h *Handler) WorkspaceRoutes(r chi.Router) {
	r.Get("/", h.get)
	r.Patch("/", h.update)
	r.Delete("/", h.delete)

	r.Get("/members", h.listMembers)
	r.Post("/members", h.addMember)
	r.Patch("/members/{userId}", h.updateMemberRole)
	r.Delete("/members/{userId}", h.removeMember)
}

// Mount registers the whole subtree with RequireWorkspace applied to the
// single-workspace branch. Callers that need a different middleware stack can
// use Routes and WorkspaceRoutes directly.
func (h *Handler) Mount(r chi.Router) {
	h.Routes(r)
	r.Route("/{workspaceId}", func(r chi.Router) {
		r.Use(RequireWorkspace(h.svc))
		h.WorkspaceRoutes(r)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	user := authctx.MustUser(r.Context())

	items, err := h.svc.ListForUser(r.Context(), user.ID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	out := make([]DTO, 0, len(items))
	for _, item := range items {
		out = append(out, NewDTO(&item.Workspace, item.Role))
	}
	httpx.List(w, out, httpx.Meta{})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	name, err := validateName(req.Name)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	user := authctx.MustUser(r.Context())
	ws, err := h.svc.Create(r.Context(), user.ID, name)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Data(w, http.StatusCreated, NewDTO(ws, authctx.RoleOwner))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())

	ws, err := h.svc.Get(r.Context(), m.WorkspaceID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, NewDTO(ws, m.Role))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	if err := authorizeWorkspaceChange(m); err != nil {
		httpx.Error(w, r, err)
		return
	}

	var req updateRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	name, err := validateName(req.Name)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	ws, err := h.svc.Update(r.Context(), m.WorkspaceID, name)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, NewDTO(ws, m.Role))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	if err := authorizeWorkspaceChange(m); err != nil {
		httpx.Error(w, r, err)
		return
	}

	if err := h.svc.Delete(r.Context(), m.WorkspaceID, m.UserID); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())

	members, err := h.svc.ListMembers(r.Context(), m.WorkspaceID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	out := make([]MemberDTO, 0, len(members))
	for _, member := range members {
		out = append(out, NewMemberDTO(member))
	}
	httpx.List(w, out, httpx.Meta{})
}

func (h *Handler) addMember(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	if !m.Role.CanManageMembers() {
		httpx.Error(w, r, httpx.ErrForbidden)
		return
	}

	var req addMemberRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	v := validate.New()
	email := v.Email("email", req.Email)
	if v.HasErrors() {
		httpx.Error(w, r, httpx.Invalid(v.Fields()))
		return
	}

	role, err := parseRole(req.Role)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	member, err := h.svc.AddMemberByEmail(r.Context(), m.WorkspaceID, email, role)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusCreated, NewMemberDTO(*member))
}

func (h *Handler) updateMemberRole(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())

	targetID, targetRole, err := h.targetMember(r, m)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := authorizeMemberChange(m, targetID, targetRole, actionUpdateRole); err != nil {
		httpx.Error(w, r, err)
		return
	}

	var req updateMemberRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	role, err := parseRole(req.Role)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	member, err := h.svc.UpdateMemberRole(r.Context(), m.WorkspaceID, targetID, role)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, NewMemberDTO(*member))
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())

	targetID, targetRole, err := h.targetMember(r, m)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := authorizeMemberChange(m, targetID, targetRole, actionRemove); err != nil {
		httpx.Error(w, r, err)
		return
	}

	if err := h.svc.RemoveMember(r.Context(), m.WorkspaceID, targetID); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// targetMember resolves {userId} to a member of the current workspace. The
// lookup is scoped to m.WorkspaceID, so an ID from another tenant reads as a
// missing member rather than leaking that the user exists.
func (h *Handler) targetMember(r *http.Request, m authctx.Membership) (uuid.UUID, authctx.Role, error) {
	targetID, err := httpx.UUIDParam(chi.URLParam(r, "userId"))
	if err != nil {
		return uuid.Nil, "", err
	}

	role, err := h.svc.MemberRole(r.Context(), m.WorkspaceID, targetID)
	if err != nil {
		return uuid.Nil, "", err
	}
	return targetID, role, nil
}

// validateName checks a workspace name and returns the trimmed value.
func validateName(name string) (string, error) {
	v := validate.New()
	trimmed := v.Required("name", name)
	if trimmed != "" {
		v.Length("name", trimmed, NameMinLength, NameMaxLength)
	}
	if v.HasErrors() {
		return "", httpx.Invalid(v.Fields())
	}
	return trimmed, nil
}
