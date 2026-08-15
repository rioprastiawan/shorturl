package link

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/security"
	"github.com/rioprastiawan/shorturl/apps/server/internal/slug"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
	"github.com/rioprastiawan/shorturl/apps/server/internal/urlx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/validate"
)

// MaxMetadataBytes caps the caller-owned metadata blob (plan §14.2).
const MaxMetadataBytes = 8 * 1024

// slugAttempts bounds retries when a generated slug collides. With a 7
// character alphabet of 56 symbols, needing more than a few is essentially
// impossible; the loop exists so a pathological case fails loudly.
const slugAttempts = 8

// Service holds link business logic.
type Service struct {
	pool   *pgxpool.Pool
	q      *store.Queries
	cache  *Cache
	cfg    config.Config
	logger *slog.Logger
}

// NewService builds the link service.
func NewService(pool *pgxpool.Pool, q *store.Queries, c *Cache, cfg config.Config, logger *slog.Logger) *Service {
	return &Service{pool: pool, q: q, cache: c, cfg: cfg, logger: logger}
}

// Cache exposes the cache helper so other features can invalidate links when
// their domain changes.
func (s *Service) Cache() *Cache { return s.cache }

// Create validates and persists a new link, then warms the redirect cache so
// the first click does not pay for a database lookup.
func (s *Service) Create(ctx context.Context, in CreateInput) (View, error) {
	v := validate.New()

	destination, err := urlx.ValidateDestination(in.DestinationURL)
	if err != nil {
		v.Add("destination_url", "%s", err.Error())
	}

	redirectType := int16(302)
	if in.RedirectType != nil {
		redirectType = *in.RedirectType
		v.Check(isValidRedirectType(redirectType), "redirect_type", "must be one of 301, 302, 307, 308")
	}

	if in.Metadata != nil {
		if len(in.Metadata) > MaxMetadataBytes {
			v.Add("metadata", "must be at most %d bytes", MaxMetadataBytes)
		} else if !json.Valid(in.Metadata) {
			v.Add("metadata", "must be a valid JSON object")
		}
	}

	if in.ExpiresAt != nil && in.ExpiresAt.Before(time.Now()) {
		v.Add("expires_at", "must be in the future")
	}
	if in.MaxClicks != nil && *in.MaxClicks < 1 {
		v.Add("max_clicks", "must be at least 1")
	}
	if in.ExternalReference != nil && len(*in.ExternalReference) > 255 {
		v.Add("external_reference", "must be at most 255 characters")
	}
	if in.Title != nil && len(*in.Title) > 255 {
		v.Add("title", "must be at most 255 characters")
	}

	domain, err := s.resolveDomain(ctx, in.WorkspaceID, in.DomainID, in.Hostname)
	if err != nil {
		if v.HasErrors() {
			return View{}, httpx.Invalid(v.Fields())
		}
		return View{}, err
	}

	// Slug: validate a caller-supplied one, otherwise generate until free.
	var chosen string
	if in.Slug != nil && strings.TrimSpace(*in.Slug) != "" {
		chosen = strings.TrimSpace(*in.Slug)
		if err := slug.Validate(chosen); err != nil {
			v.Add("slug", "%s", err.Error())
		}
	}

	if v.HasErrors() {
		return View{}, httpx.Invalid(v.Fields())
	}

	if chosen == "" {
		chosen, err = s.generateFreeSlug(ctx, domain.ID)
		if err != nil {
			return View{}, err
		}
	}

	var passwordHash *string
	if in.Password != nil && *in.Password != "" {
		hashed, err := security.HashPassword(*in.Password)
		if err != nil {
			return View{}, httpx.Internal(err)
		}
		passwordHash = &hashed
	}

	createdVia := in.CreatedVia
	if createdVia == "" {
		createdVia = "dashboard"
	}

	row, err := s.q.CreateLink(ctx, store.CreateLinkParams{
		WorkspaceID:       in.WorkspaceID,
		DomainID:          domain.ID,
		Slug:              chosen,
		DestinationUrl:    destination,
		Title:             in.Title,
		Status:            "active",
		RedirectType:      redirectType,
		PasswordHash:      passwordHash,
		ExpiresAt:         in.ExpiresAt,
		MaxClicks:         in.MaxClicks,
		ExternalReference: in.ExternalReference,
		Metadata:          in.Metadata,
		CreatedBy:         in.CreatedBy,
		CreatedVia:        createdVia,
	})
	if err != nil {
		return View{}, s.mapCreateError(err, in.Slug != nil)
	}

	view := View{Link: row, Hostname: domain.Hostname}
	s.warm(ctx, view)
	return view, nil
}

// Get returns a link scoped to the workspace.
func (s *Service) Get(ctx context.Context, workspaceID, linkID uuid.UUID) (View, error) {
	row, err := s.q.GetLinkInWorkspace(ctx, store.GetLinkInWorkspaceParams{
		ID:          linkID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return View{}, httpx.ErrNotFound
		}
		return View{}, httpx.Internal(err)
	}
	return View{Link: row.Link, Hostname: row.Hostname}, nil
}

// GetByExternalReference answers "have I already shortened this?".
func (s *Service) GetByExternalReference(ctx context.Context, workspaceID uuid.UUID, ref string) (View, error) {
	row, err := s.q.GetLinkByExternalReference(ctx, store.GetLinkByExternalReferenceParams{
		WorkspaceID:       workspaceID,
		ExternalReference: &ref,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return View{}, httpx.ErrNotFound
		}
		return View{}, httpx.Internal(err)
	}
	return View{Link: row.Link, Hostname: row.Hostname}, nil
}

// Update applies a partial change and invalidates both the old and the new
// cache entries, because a slug or domain change moves the key.
func (s *Service) Update(ctx context.Context, workspaceID, linkID uuid.UUID, in UpdateInput) (View, error) {
	before, err := s.Get(ctx, workspaceID, linkID)
	if err != nil {
		return View{}, err
	}

	v := validate.New()
	params := store.UpdateLinkParams{
		ID:          linkID,
		WorkspaceID: workspaceID,
	}

	if in.DestinationURL != nil {
		destination, err := urlx.ValidateDestination(*in.DestinationURL)
		if err != nil {
			v.Add("destination_url", "%s", err.Error())
		} else {
			params.DestinationUrl = &destination
		}
	}
	if in.Status != nil {
		v.Check(isValidStatus(*in.Status), "status", "must be one of active, disabled, archived")
		params.Status = in.Status
	}
	if in.RedirectType != nil {
		v.Check(isValidRedirectType(*in.RedirectType), "redirect_type", "must be one of 301, 302, 307, 308")
		params.RedirectType = in.RedirectType
	}
	if in.Slug != nil {
		trimmed := strings.TrimSpace(*in.Slug)
		if err := slug.Validate(trimmed); err != nil {
			v.Add("slug", "%s", err.Error())
		} else {
			params.Slug = &trimmed
		}
	}
	if in.ExpiresAt != nil && in.ExpiresAt.Before(time.Now()) {
		v.Add("expires_at", "must be in the future")
	}
	if in.MaxClicks != nil && *in.MaxClicks < 1 {
		v.Add("max_clicks", "must be at least 1")
	}
	if in.SetMetadata && in.Metadata != nil {
		if len(in.Metadata) > MaxMetadataBytes {
			v.Add("metadata", "must be at most %d bytes", MaxMetadataBytes)
		} else if !json.Valid(in.Metadata) {
			v.Add("metadata", "must be a valid JSON object")
		}
	}

	targetHostname := before.Hostname
	if in.DomainID != nil || in.Hostname != nil {
		domain, err := s.resolveDomain(ctx, workspaceID, in.DomainID, in.Hostname)
		if err != nil {
			if v.HasErrors() {
				return View{}, httpx.Invalid(v.Fields())
			}
			return View{}, err
		}
		params.DomainID = &domain.ID
		targetHostname = domain.Hostname
	}

	if v.HasErrors() {
		return View{}, httpx.Invalid(v.Fields())
	}

	params.SetTitle = in.SetTitle
	params.Title = in.Title
	params.SetExpiresAt = in.SetExpiresAt
	params.ExpiresAt = in.ExpiresAt
	params.SetMaxClicks = in.SetMaxClicks
	params.MaxClicks = in.MaxClicks
	params.SetMetadata = in.SetMetadata
	params.Metadata = in.Metadata

	params.SetPassword = in.SetPassword
	if in.SetPassword {
		if in.Password != nil && *in.Password != "" {
			hashed, err := security.HashPassword(*in.Password)
			if err != nil {
				return View{}, httpx.Internal(err)
			}
			params.PasswordHash = &hashed
		} else {
			params.PasswordHash = nil // explicit clear
		}
	}

	row, err := s.q.UpdateLink(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return View{}, httpx.ErrNotFound
		}
		return View{}, s.mapCreateError(err, in.Slug != nil)
	}

	// Drop the old key first: if the slug or domain moved, leaving it behind
	// would keep serving the previous destination until the TTL expired.
	s.cache.InvalidateLink(ctx, before.Hostname, before.Link.Slug, before.Link.ID)

	view := View{Link: row, Hostname: targetHostname}
	s.warm(ctx, view)
	return view, nil
}

// Delete removes a link and its cache entry.
func (s *Service) Delete(ctx context.Context, workspaceID, linkID uuid.UUID) error {
	before, err := s.Get(ctx, workspaceID, linkID)
	if err != nil {
		return err
	}

	affected, err := s.q.DeleteLink(ctx, store.DeleteLinkParams{ID: linkID, WorkspaceID: workspaceID})
	if err != nil {
		return httpx.Internal(err)
	}
	if affected == 0 {
		return httpx.ErrNotFound
	}

	s.cache.InvalidateLink(ctx, before.Hostname, before.Link.Slug, before.Link.ID)
	return nil
}

// warm writes the redirect cache entry for an active link. A disabled or
// expired link is cached too, so the redirect path can answer 410 without
// touching PostgreSQL.
func (s *Service) warm(ctx context.Context, v View) {
	entry := Cached{
		LinkID:       v.Link.ID,
		WorkspaceID:  v.Link.WorkspaceID,
		URL:          v.Link.DestinationUrl,
		RedirectType: int(v.Link.RedirectType),
		ExpiresAt:    v.Link.ExpiresAt,
		MaxClicks:    v.Link.MaxClicks,
		HasPassword:  v.Link.PasswordHash != nil && *v.Link.PasswordHash != "",
		Disabled:     v.Link.Status != "active",
	}
	s.cache.Set(ctx, v.Hostname, v.Link.Slug, v.Link.DomainID, entry)

	if v.Link.MaxClicks != nil {
		s.cache.SeedClickCounter(ctx, v.Link.ID, v.Link.ClickCount)
	}
}

// resolveDomain picks the target domain: explicit ID, explicit hostname, or
// the workspace default. Only active domains are accepted, so a link can never
// be created on a domain that would not serve it.
func (s *Service) resolveDomain(ctx context.Context, workspaceID uuid.UUID, domainID *uuid.UUID, hostname *string) (store.Domain, error) {
	switch {
	case domainID != nil:
		d, err := s.q.GetDomainInWorkspace(ctx, store.GetDomainInWorkspaceParams{
			ID: *domainID, WorkspaceID: workspaceID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return store.Domain{}, httpx.Invalid(map[string][]string{
					"domain_id": {"no such domain in this workspace"},
				})
			}
			return store.Domain{}, httpx.Internal(err)
		}
		return s.requireActive(d)

	case hostname != nil:
		normalized := urlx.NormalizeHostname(*hostname)
		d, err := s.q.GetDomainByHostnameInWorkspace(ctx, store.GetDomainByHostnameInWorkspaceParams{
			Hostname: normalized, WorkspaceID: workspaceID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return store.Domain{}, httpx.Invalid(map[string][]string{
					"domain": {fmt.Sprintf("%q is not a domain in this workspace", normalized)},
				})
			}
			return store.Domain{}, httpx.Internal(err)
		}
		return s.requireActive(d)

	default:
		d, err := s.q.GetDefaultDomain(ctx, workspaceID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// The most common integration failure: a caller sends only
				// destination_url before any domain is connected. Say exactly
				// what to do about it.
				return store.Domain{}, httpx.Conflictf("no_default_domain",
					"This workspace has no verified default domain. Add and verify a domain, or pass an explicit domain.")
			}
			return store.Domain{}, httpx.Internal(err)
		}
		return d, nil
	}
}

func (s *Service) requireActive(d store.Domain) (store.Domain, error) {
	if d.Status != "active" {
		return store.Domain{}, httpx.Conflictf("domain_not_active",
			"Domain %s is %s. Verify it before creating links on it.", d.Hostname, d.Status)
	}
	return d, nil
}

func (s *Service) generateFreeSlug(ctx context.Context, domainID uuid.UUID) (string, error) {
	for range slugAttempts {
		candidate, err := slug.Generate(slug.DefaultLength)
		if err != nil {
			return "", httpx.Internal(err)
		}
		exists, err := s.q.SlugExistsOnDomain(ctx, store.SlugExistsOnDomainParams{
			DomainID: domainID, Slug: candidate,
		})
		if err != nil {
			return "", httpx.Internal(err)
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", httpx.Internal(errors.New("could not find a free slug after several attempts"))
}

// mapCreateError turns unique-violation errors into the right client message.
// Which constraint fired matters: a slug clash is the caller's choice of slug,
// an external_reference clash means they already created this link.
func (s *Service) mapCreateError(err error, slugWasExplicit bool) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return httpx.Internal(err)
	}

	switch pgErr.ConstraintName {
	case "links_domain_slug_unique":
		if slugWasExplicit {
			return httpx.Conflictf("slug_taken", "That slug is already in use on this domain")
		}
		return httpx.Internal(fmt.Errorf("generated slug collided: %w", err))
	case "links_workspace_external_reference_unique":
		return httpx.Conflictf("external_reference_taken",
			"A link with that external_reference already exists in this workspace")
	default:
		return httpx.Internal(err)
	}
}

func isValidRedirectType(t int16) bool {
	return t == 301 || t == 302 || t == 307 || t == 308
}

func isValidStatus(s string) bool {
	return s == "active" || s == "disabled" || s == "archived"
}

// encodeCursor and decodeCursor implement keyset pagination over
// (created_at, id). Offset pagination would drift as links are created during
// a walk and gets slower the deeper it goes.
func encodeCursor(createdAt time.Time, id uuid.UUID) string {
	return encodeCursorRaw(createdAt.UTC().Format(time.RFC3339Nano) + "|" + id.String())
}

func encodeCursorRaw(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func decodeCursor(raw string) (time.Time, uuid.UUID, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("malformed cursor")
	}
	before, after, found := strings.Cut(string(decoded), "|")
	if !found {
		return time.Time{}, uuid.Nil, fmt.Errorf("malformed cursor")
	}
	ts, err := time.Parse(time.RFC3339Nano, before)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("malformed cursor")
	}
	id, err := uuid.Parse(after)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("malformed cursor")
	}
	return ts, id, nil
}
