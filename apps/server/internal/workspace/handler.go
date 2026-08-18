package workspace

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/validate"
)

// Handler is the HTTP layer over Service.
type Handler struct {
	svc   *Service
	audit AuditRecorder
}

type AuditRecorder interface {
	RecordAudit(context.Context, uuid.UUID, *uuid.UUID, string, string, uuid.UUID, string, json.RawMessage)
}

const memberListPageSize = 10

// NewHandler builds the workspace handler.
func NewHandler(svc *Service, recorders ...AuditRecorder) *Handler {
	h := &Handler{svc: svc}
	if len(recorders) > 0 {
		h.audit = recorders[0]
	}
	return h
}

func (h *Handler) record(r *http.Request, workspaceID, actorID uuid.UUID, action, entityType string, entityID uuid.UUID, label string) {
	if h.audit != nil {
		h.audit.RecordAudit(r.Context(), workspaceID, &actorID, action, entityType, entityID, label, nil)
	}
}

// Routes registers the collection endpoints, which need an authenticated user
// but no workspace.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Post("/demo", h.createDemo)
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
	r.Post("/invitations", h.createInvitation)
	r.Get("/invitations", h.listInvitations)
	r.Post("/invitations/{invitationId}/renew", h.renewInvitation)
	r.Delete("/invitations/{invitationId}", h.revokeInvitation)
	r.Patch("/members/{userId}", h.updateMemberRole)
	r.Delete("/members/{userId}", h.removeMember)
}

func (h *Handler) listInvitations(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	if !m.Role.CanManageMembers() {
		httpx.Error(w, r, httpx.ErrForbidden)
		return
	}
	page := httpx.QueryInt(r, "page", 1, 1, 1_000_000)
	perPage := httpx.QueryInt(r, "per_page", memberListPageSize, 1, 100)
	items, total, err := h.svc.ListInvitations(r.Context(), m.WorkspaceID, perPage, (page-1)*perPage)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.List(w, items, httpx.Meta{Total: &total})
}

func (h *Handler) invitationID(w http.ResponseWriter, r *http.Request) (authctx.Membership, uuid.UUID, bool) {
	m := authctx.MustMembership(r.Context())
	if !m.Role.CanManageMembers() {
		httpx.Error(w, r, httpx.ErrForbidden)
		return m, uuid.Nil, false
	}
	id, err := httpx.UUIDParam(chi.URLParam(r, "invitationId"))
	if err != nil {
		httpx.Error(w, r, err)
		return m, uuid.Nil, false
	}
	return m, id, true
}

func (h *Handler) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	m, id, ok := h.invitationID(w, r)
	if !ok {
		return
	}
	if err := h.svc.RevokeInvitation(r.Context(), m.WorkspaceID, id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.record(r, m.WorkspaceID, m.UserID, "invitation.revoked", "invitation", id, "Workspace invitation")
	httpx.NoContent(w)
}

func (h *Handler) renewInvitation(w http.ResponseWriter, r *http.Request) {
	m, id, ok := h.invitationID(w, r)
	if !ok {
		return
	}
	invitation, err := h.svc.RenewInvitation(r.Context(), m.WorkspaceID, id, m.UserID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.record(r, m.WorkspaceID, m.UserID, "invitation.renewed", "invitation", invitation.ID, string(invitation.Role)+" invitation")
	httpx.Data(w, http.StatusCreated, invitation)
}

func (h *Handler) createInvitation(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	if !m.Role.CanManageMembers() {
		httpx.Error(w, r, httpx.ErrForbidden)
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	role, err := parseRole(req.Role)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	invitation, err := h.svc.CreateInvitation(r.Context(), m.WorkspaceID, m.UserID, role)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.record(r, m.WorkspaceID, m.UserID, "invitation.created", "invitation", invitation.ID, string(invitation.Role)+" invitation")
	httpx.Data(w, http.StatusCreated, invitation)
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
	limit := httpx.QueryInt(r, "limit", 20, 1, 100)

	items, nextCursor, err := h.svc.ListForUser(r.Context(), user.ID, r.URL.Query().Get("search"), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	out := make([]DTO, 0, len(items))
	for _, item := range items {
		out = append(out, NewDTO(&item.Workspace, item.Role))
	}
	httpx.List(w, out, httpx.Meta{NextCursor: nextCursor})
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
	h.record(r, ws.ID, user.ID, "workspace.created", "workspace", ws.ID, ws.Name)

	httpx.Data(w, http.StatusCreated, NewDTO(ws, authctx.RoleOwner))
}

func (h *Handler) createDemo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Size DemoSize `json:"size"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if req.Size == "" {
		req.Size = DemoStarter
	}
	if _, _, ok := DemoEstimate(req.Size); !ok {
		httpx.Error(w, r, httpx.BadRequest("size must be one of starter, busy, five_year"))
		return
	}
	user := authctx.MustUser(r.Context())
	if req.Size != DemoStarter && !user.IsAdmin {
		httpx.Error(w, r, httpx.ErrForbidden)
		return
	}
	ws, err := h.svc.CreateDemo(r.Context(), user.ID, req.Size)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.record(r, ws.ID, user.ID, "workspace.created", "workspace", ws.ID, ws.Name)
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
	h.record(r, m.WorkspaceID, m.UserID, "workspace.updated", "workspace", ws.ID, ws.Name)
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

	page := httpx.QueryInt(r, "page", 1, 1, 1_000_000)
	perPage := httpx.QueryInt(r, "per_page", memberListPageSize, 1, 100)
	members, total, err := h.svc.ListMembers(r.Context(), m.WorkspaceID, perPage, (page-1)*perPage)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	out := make([]MemberDTO, 0, len(members))
	for _, member := range members {
		out = append(out, NewMemberDTO(member))
	}
	httpx.List(w, out, httpx.Meta{Total: &total})
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
	h.record(r, m.WorkspaceID, m.UserID, "member.added", "member", member.UserID, member.Email)
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
	h.record(r, m.WorkspaceID, m.UserID, "member.role_updated", "member", member.UserID, member.Email)
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
	action := "member.removed"
	if targetID == m.UserID {
		action = "member.left"
	}
	h.record(r, m.WorkspaceID, m.UserID, action, "member", targetID, targetID.String())
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
