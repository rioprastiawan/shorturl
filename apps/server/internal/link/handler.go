package link

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
)

// verifyPassword is a package-level indirection so resolve.go can call the
// security package without link importing it twice.
var verifyPassword = securityVerify

// Handler exposes the dashboard link endpoints.
type Handler struct {
	svc *Service
}

// NewHandler builds the handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes registers the workspace-scoped link endpoints.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Post("/preview", h.preview)
	r.Get("/tags", h.listTags)
	r.Get("/audit-log", h.listAuditLog)
	r.Get("/{linkId}", h.get)
	r.Patch("/{linkId}", h.update)
	r.Delete("/{linkId}", h.delete)
}

func (h *Handler) listAuditLog(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	page := httpx.QueryInt(r, "page", 1, 1, 1_000_000)
	perPage := httpx.QueryInt(r, "per_page", 25, 1, 100)
	items, total, err := h.svc.ListAuditLog(r.Context(), m.WorkspaceID, perPage, (page-1)*perPage)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.List(w, items, httpx.Meta{Total: &total})
}

func (h *Handler) listTags(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	tags, err := h.svc.ListTags(r.Context(), m.WorkspaceID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.List(w, tags, httpx.Meta{})
}

type previewRequest struct {
	DestinationURL string `json:"destination_url"`
}

func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	if !m.Role.CanManageLinks() {
		httpx.Error(w, r, httpx.ErrForbidden)
		return
	}
	var req previewRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	preview, err := h.svc.Preview(r.Context(), req.DestinationURL)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, preview)
}

type createRequest struct {
	DestinationURL    string          `json:"destination_url"`
	DomainID          *uuid.UUID      `json:"domain_id"`
	Domain            *string         `json:"domain"`
	Slug              *string         `json:"slug"`
	Title             *string         `json:"title"`
	RedirectType      *int16          `json:"redirect_type"`
	Password          *string         `json:"password"`
	ExpiresAt         *time.Time      `json:"expires_at"`
	MaxClicks         *int64          `json:"max_clicks"`
	ExternalReference *string         `json:"external_reference"`
	Metadata          json.RawMessage `json:"metadata"`
}

func (r createRequest) toInput(workspaceID uuid.UUID, createdBy *uuid.UUID, via string) CreateInput {
	return CreateInput{
		WorkspaceID:       workspaceID,
		DomainID:          r.DomainID,
		Hostname:          r.Domain,
		DestinationURL:    r.DestinationURL,
		Slug:              r.Slug,
		Title:             r.Title,
		RedirectType:      r.RedirectType,
		Password:          r.Password,
		ExpiresAt:         r.ExpiresAt,
		MaxClicks:         r.MaxClicks,
		ExternalReference: r.ExternalReference,
		Metadata:          r.Metadata,
		CreatedBy:         createdBy,
		CreatedVia:        via,
	}
}

type updateRequest struct {
	DestinationURL *string          `json:"destination_url"`
	Status         *string          `json:"status"`
	RedirectType   *int16           `json:"redirect_type"`
	Slug           *string          `json:"slug"`
	DomainID       *uuid.UUID       `json:"domain_id"`
	Domain         *string          `json:"domain"`
	Title          *json.RawMessage `json:"title"`
	ExpiresAt      *json.RawMessage `json:"expires_at"`
	MaxClicks      *json.RawMessage `json:"max_clicks"`
	Password       *json.RawMessage `json:"password"`
	Metadata       *json.RawMessage `json:"metadata"`
}

// toInput distinguishes an absent field from an explicit null.
//
// Nullable fields arrive as *json.RawMessage: nil means the key was absent and
// the column must not change, whereas a present `null` means clear it. A plain
// *string cannot express that difference, and without it a caller could never
// remove a link's expiry.
func (r updateRequest) toInput() (UpdateInput, error) {
	in := UpdateInput{
		DestinationURL: r.DestinationURL,
		Status:         r.Status,
		RedirectType:   r.RedirectType,
		Slug:           r.Slug,
		DomainID:       r.DomainID,
		Hostname:       r.Domain,
	}

	if r.Title != nil {
		in.SetTitle = true
		if !isJSONNull(*r.Title) {
			var v string
			if err := json.Unmarshal(*r.Title, &v); err != nil {
				return in, httpx.Invalid(map[string][]string{"title": {"must be a string or null"}})
			}
			in.Title = &v
		}
	}
	if r.ExpiresAt != nil {
		in.SetExpiresAt = true
		if !isJSONNull(*r.ExpiresAt) {
			var v time.Time
			if err := json.Unmarshal(*r.ExpiresAt, &v); err != nil {
				return in, httpx.Invalid(map[string][]string{"expires_at": {"must be an RFC3339 timestamp or null"}})
			}
			in.ExpiresAt = &v
		}
	}
	if r.MaxClicks != nil {
		in.SetMaxClicks = true
		if !isJSONNull(*r.MaxClicks) {
			var v int64
			if err := json.Unmarshal(*r.MaxClicks, &v); err != nil {
				return in, httpx.Invalid(map[string][]string{"max_clicks": {"must be a number or null"}})
			}
			in.MaxClicks = &v
		}
	}
	if r.Password != nil {
		in.SetPassword = true
		if !isJSONNull(*r.Password) {
			var v string
			if err := json.Unmarshal(*r.Password, &v); err != nil {
				return in, httpx.Invalid(map[string][]string{"password": {"must be a string or null"}})
			}
			in.Password = &v
		}
	}
	if r.Metadata != nil {
		in.SetMetadata = true
		if !isJSONNull(*r.Metadata) {
			in.Metadata = *r.Metadata
		}
	}

	return in, nil
}

func isJSONNull(raw json.RawMessage) bool {
	return string(raw) == "null"
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())

	in := ListInput{
		WorkspaceID:       m.WorkspaceID,
		Search:            r.URL.Query().Get("search"),
		Tag:               r.URL.Query().Get("tag"),
		Status:            r.URL.Query().Get("status"),
		ExternalReference: r.URL.Query().Get("external_reference"),
		Cursor:            r.URL.Query().Get("cursor"),
		Limit:             httpx.QueryInt(r, "limit", DefaultListLimit, 1, MaxListLimit),
	}

	if raw := r.URL.Query().Get("domain_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.Error(w, r, httpx.BadRequest("domain_id must be a UUID"))
			return
		}
		in.DomainID = &id
	}
	var err error
	if in.CreatedAfter, err = parseTimeQuery(r, "created_after"); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if in.CreatedBefore, err = parseTimeQuery(r, "created_before"); err != nil {
		httpx.Error(w, r, err)
		return
	}

	views, cursor, err := h.svc.List(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items := make([]DTO, 0, len(views))
	for _, v := range views {
		items = append(items, ToDTO(v))
	}
	httpx.List(w, items, httpx.Meta{NextCursor: cursor})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	if !m.Role.CanManageLinks() {
		httpx.Error(w, r, httpx.ErrForbidden)
		return
	}

	var req createRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	user := authctx.MustUser(r.Context())
	view, err := h.svc.Create(r.Context(), req.toInput(m.WorkspaceID, &user.ID, "dashboard"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.svc.RecordAudit(r.Context(), m.WorkspaceID, &user.ID, "link.created", view.Link.ID, view.ShortURL(), nil)
	httpx.Data(w, http.StatusCreated, ToDTO(view))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	id, err := httpx.UUIDParam(chi.URLParam(r, "linkId"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	view, err := h.svc.Get(r.Context(), m.WorkspaceID, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, ToDTO(view))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	if !m.Role.CanManageLinks() {
		httpx.Error(w, r, httpx.ErrForbidden)
		return
	}

	id, err := httpx.UUIDParam(chi.URLParam(r, "linkId"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var req updateRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	in, err := req.toInput()
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	view, err := h.svc.Update(r.Context(), m.WorkspaceID, id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	user := authctx.MustUser(r.Context())
	action := "link.updated"
	if in.Status != nil {
		action = "link." + *in.Status
	}
	h.svc.RecordAudit(r.Context(), m.WorkspaceID, &user.ID, action, view.Link.ID, view.ShortURL(), nil)
	httpx.Data(w, http.StatusOK, ToDTO(view))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	m := authctx.MustMembership(r.Context())
	if !m.Role.CanManageLinks() {
		httpx.Error(w, r, httpx.ErrForbidden)
		return
	}

	id, err := httpx.UUIDParam(chi.URLParam(r, "linkId"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	before, err := h.svc.Get(r.Context(), m.WorkspaceID, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Delete(r.Context(), m.WorkspaceID, id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	user := authctx.MustUser(r.Context())
	h.svc.RecordAudit(r.Context(), m.WorkspaceID, &user.ID, "link.deleted", id, before.ShortURL(), nil)
	httpx.NoContent(w)
}

func parseTimeQuery(r *http.Request, key string) (*time.Time, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, httpx.BadRequest(key + " must be an RFC3339 timestamp")
	}
	return &t, nil
}
