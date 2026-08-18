// Package publicapi is the API-key-authenticated REST surface that internal
// systems integrate against (plan §14).
//
// Two rules hold everywhere in here:
//
//   - The workspace comes from the API key, never from the request. A key
//     issued for one workspace cannot see or touch another.
//   - Responses are rendered with link.ToDTO, the same function the dashboard
//     uses, so the two views of a link can never drift apart.
package publicapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rioprastiawan/shorturl/apps/server/internal/apikey"
	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/link"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

// statusDisabled is what DELETE sets by default: plan §14.7 prefers soft
// disabling so click history and analytics survive the link.
const statusDisabled = "disabled"

// Handler serves the public link API.
type Handler struct {
	links *link.Service
	q     *store.Queries
}

// NewHandler builds the handler.
func NewHandler(links *link.Service, q *store.Queries) *Handler {
	return &Handler{links: links, q: q}
}

// Routes registers the public link endpoints. The orchestrator mounts this at
// /api/v1/links behind apikey.RequireAPIKey and RateLimitByAPIKey; the scope
// gates below assume a principal is already on the context.
func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(apikey.RequireScope(authctx.ScopeLinksRead))

		r.Get("/", h.list)
		// Registered before /{linkId} so the literal segment wins the match and
		// "by-reference" is never parsed as a link ID.
		r.Get("/by-reference/{externalReference}", h.lookup)
		r.Get("/{linkId}", h.get)
	})

	r.Group(func(r chi.Router) {
		r.Use(apikey.RequireScope(authctx.ScopeLinksWrite))

		r.Post("/", h.create)
		r.Patch("/{linkId}", h.update)
		r.Delete("/{linkId}", h.disable)
	})
}

// create implements POST /links, including idempotent retries (plan §14.3).
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	principal := authctx.MustAPIKey(r.Context())

	// The body is read whole first because idempotency fingerprints the bytes
	// the caller actually sent, and httpx.Decode consumes the stream.
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpx.MaxBodyBytes))
	if err != nil {
		httpx.Error(w, r, httpx.BadRequest("Request body is too large"))
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))

	var req createRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	idempotencyKey := r.Header.Get(IdempotencyHeader)
	if idempotencyKey == "" {
		view, err := h.links.Create(r.Context(), req.toInput(principal.WorkspaceID))
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		h.links.RecordAudit(r.Context(), principal.WorkspaceID, nil, "link.created", "link", view.Link.ID, view.ShortURL(), nil)
		httpx.Data(w, http.StatusCreated, link.ToDTO(view))
		return
	}

	h.createIdempotent(w, r, principal.WorkspaceID, idempotencyKey, raw, req)
}

func (h *Handler) createIdempotent(
	w http.ResponseWriter,
	r *http.Request,
	workspaceID uuid.UUID,
	key string,
	raw []byte,
	req createRequest,
) {
	if !ValidIdempotencyKey(key) {
		httpx.Error(w, r, httpx.BadRequest("Idempotency-Key must be at most 255 characters"))
		return
	}

	requestHash, err := CanonicalHash(raw)
	if err != nil {
		httpx.Error(w, r, httpx.BadRequest("Request body must be a JSON object"))
		return
	}

	ctx := r.Context()
	record, err := h.loadRecord(ctx, workspaceID, key)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	switch Decide(record, requestHash) {
	case DecisionConflict:
		httpx.Error(w, r, errIdempotencyConflict())
		return
	case DecisionLinkGone:
		httpx.Error(w, r, errIdempotencyLinkGone())
		return
	case DecisionReplay:
		h.replay(w, r, workspaceID, *record.LinkID)
		return
	}

	view, err := h.links.Create(ctx, req.toInput(workspaceID))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	linkID := view.Link.ID
	_, err = h.q.CreateIdempotencyRecord(ctx, store.CreateIdempotencyRecordParams{
		WorkspaceID:    workspaceID,
		IdempotencyKey: key,
		RequestHash:    requestHash,
		LinkID:         &linkID,
		ExpiresAt:      time.Now().Add(IdempotencyTTL),
	})
	switch {
	case err == nil:
		h.links.RecordAudit(ctx, workspaceID, nil, "link.created", "link", view.Link.ID, view.ShortURL(), nil)
		httpx.Data(w, http.StatusCreated, link.ToDTO(view))
	case isUniqueViolation(err):
		// Two identical requests raced past the lookup and both created a link.
		// This one lost: hand back the winner's link and remove the duplicate,
		// or the caller ends up with two short URLs for one retry.
		h.resolveRace(w, r, workspaceID, key, requestHash, linkID)
	default:
		httpx.Error(w, r, httpx.Internal(err))
	}
}

// resolveRace settles a lost insert race: return the winning link and delete
// the duplicate this request created.
func (h *Handler) resolveRace(
	w http.ResponseWriter,
	r *http.Request,
	workspaceID uuid.UUID,
	key, requestHash string,
	loserID uuid.UUID,
) {
	// Detached: the duplicate must go even if the client has already hung up.
	cleanup := context.WithoutCancel(r.Context())
	defer func() {
		_ = h.links.Delete(cleanup, workspaceID, loserID)
	}()

	record, err := h.loadRecord(r.Context(), workspaceID, key)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	switch Decide(record, requestHash) {
	case DecisionReplay:
		h.replay(w, r, workspaceID, *record.LinkID)
	case DecisionConflict:
		httpx.Error(w, r, errIdempotencyConflict())
	default:
		// The winner's record vanished between the failed insert and this read
		// (expiry, or its link was deleted). Nothing left to replay.
		httpx.Error(w, r, errIdempotencyLinkGone())
	}
}

// replay returns the link a previous request created, with 200 rather than 201
// so the caller can tell a retry from a first success (plan §14.9).
func (h *Handler) replay(w http.ResponseWriter, r *http.Request, workspaceID, linkID uuid.UUID) {
	view, err := h.links.Get(r.Context(), workspaceID, linkID)
	if err != nil {
		if errors.Is(err, httpx.ErrNotFound) {
			httpx.Error(w, r, errIdempotencyLinkGone())
			return
		}
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, link.ToDTO(view))
}

// loadRecord reads the stored idempotency state. The query already filters out
// expired rows, so "not found" and "expired" are the same thing here.
func (h *Handler) loadRecord(ctx context.Context, workspaceID uuid.UUID, key string) (Record, error) {
	row, err := h.q.GetIdempotencyRecord(ctx, store.GetIdempotencyRecordParams{
		WorkspaceID:    workspaceID,
		IdempotencyKey: key,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Record{}, nil
		}
		return Record{}, httpx.Internal(err)
	}
	return Record{Found: true, RequestHash: row.RequestHash, LinkID: row.LinkID}, nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	principal := authctx.MustAPIKey(r.Context())

	in := link.ListInput{
		WorkspaceID:       principal.WorkspaceID,
		Search:            r.URL.Query().Get("search"),
		Status:            r.URL.Query().Get("status"),
		ExternalReference: r.URL.Query().Get("external_reference"),
		Cursor:            r.URL.Query().Get("cursor"),
		Limit:             httpx.QueryInt(r, "limit", link.DefaultListLimit, 1, link.MaxListLimit),
	}

	if rawID := r.URL.Query().Get("domain_id"); rawID != "" {
		id, err := uuid.Parse(rawID)
		if err != nil {
			httpx.Error(w, r, httpx.BadRequest("domain_id must be a UUID"))
			return
		}
		in.DomainID = &id
	}

	var err error
	if in.CreatedAfter, err = timeQuery(r, "created_after"); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if in.CreatedBefore, err = timeQuery(r, "created_before"); err != nil {
		httpx.Error(w, r, err)
		return
	}

	views, cursor, err := h.links.List(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items := make([]link.DTO, 0, len(views))
	for _, v := range views {
		items = append(items, link.ToDTO(v))
	}
	httpx.List(w, items, httpx.Meta{NextCursor: cursor})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	principal := authctx.MustAPIKey(r.Context())

	id, err := httpx.UUIDParam(chi.URLParam(r, "linkId"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	view, err := h.links.Get(r.Context(), principal.WorkspaceID, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, link.ToDTO(view))
}

// lookup answers "have I already shortened this?" for a caller-owned
// identifier (plan §14.5).
func (h *Handler) lookup(w http.ResponseWriter, r *http.Request) {
	principal := authctx.MustAPIKey(r.Context())

	ref := chi.URLParam(r, "externalReference")
	if ref == "" {
		httpx.Error(w, r, httpx.ErrNotFound)
		return
	}

	view, err := h.links.GetByExternalReference(r.Context(), principal.WorkspaceID, ref)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, link.ToDTO(view))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	principal := authctx.MustAPIKey(r.Context())

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

	view, err := h.links.Update(r.Context(), principal.WorkspaceID, id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.links.RecordAudit(r.Context(), principal.WorkspaceID, nil, "link.updated", "link", view.Link.ID, view.ShortURL(), nil)
	httpx.Data(w, http.StatusOK, link.ToDTO(view))
}

// disable soft-disables by default. Plan §14.7 prefers that for integrations:
// a mistaken DELETE from an automated system is then recoverable, and the
// link's analytics are not lost with it. ?hard=true opts into a real delete.
func (h *Handler) disable(w http.ResponseWriter, r *http.Request) {
	principal := authctx.MustAPIKey(r.Context())

	id, err := httpx.UUIDParam(chi.URLParam(r, "linkId"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if r.URL.Query().Get("hard") == "true" {
		before, err := h.links.Get(r.Context(), principal.WorkspaceID, id)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		if err := h.links.Delete(r.Context(), principal.WorkspaceID, id); err != nil {
			httpx.Error(w, r, err)
			return
		}
		h.links.RecordAudit(r.Context(), principal.WorkspaceID, nil, "link.deleted", "link", id, before.ShortURL(), nil)
		httpx.NoContent(w)
		return
	}

	status := statusDisabled
	view, err := h.links.Update(r.Context(), principal.WorkspaceID, id, link.UpdateInput{Status: &status})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.links.RecordAudit(r.Context(), principal.WorkspaceID, nil, "link.disabled", "link", id, view.ShortURL(), nil)
	httpx.NoContent(w)
}

func timeQuery(r *http.Request, key string) (*time.Time, error) {
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

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func errIdempotencyConflict() error {
	return httpx.Conflictf("idempotency_conflict",
		"The idempotency key was already used with a different request")
}

func errIdempotencyLinkGone() error {
	return httpx.Conflictf("idempotency_link_gone",
		"The link created with this key no longer exists")
}
