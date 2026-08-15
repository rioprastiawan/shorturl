package link

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

// Resolution is the redirect engine's answer for one request.
type Resolution struct {
	Status      ResolutionStatus
	LinkID      uuid.UUID
	WorkspaceID uuid.UUID
	URL         string
	Code        int
	NeedsAuth   bool
}

// ResolutionStatus tells the handler what to do.
type ResolutionStatus int

const (
	// StatusRedirect is the happy path.
	StatusRedirect ResolutionStatus = iota
	// StatusNotFound: no such host+slug.
	StatusNotFound
	// StatusGone: the link existed but is disabled, expired, or exhausted.
	StatusGone
	// StatusPassword: the link needs a password before it will resolve.
	StatusPassword
)

// Resolve looks up a link for the redirect path.
//
// Order matters for latency: Redis first, PostgreSQL only on a miss, and the
// result of a miss is always written back — including "not found", so a scan
// for random slugs cannot turn into a database query per request.
func (s *Service) Resolve(ctx context.Context, hostname, linkSlug string) Resolution {
	entry, err := s.cache.Get(ctx, hostname, linkSlug)
	if err != nil {
		// A Redis failure must degrade to PostgreSQL, never to an error page.
		s.logger.Warn("cache read failed, falling back to database",
			slog.String("hostname", hostname),
			slog.String("error", err.Error()),
		)
		entry = nil
	}

	if entry == nil {
		entry = s.loadFromDatabase(ctx, hostname, linkSlug)
		if entry == nil {
			return Resolution{Status: StatusNotFound}
		}
	}

	if entry.NotFound {
		return Resolution{Status: StatusNotFound}
	}

	return s.evaluate(ctx, *entry)
}

// evaluate applies the redirect rules from plan §10 to a resolved entry.
func (s *Service) evaluate(ctx context.Context, entry Cached) Resolution {
	base := Resolution{
		LinkID:      entry.LinkID,
		WorkspaceID: entry.WorkspaceID,
		URL:         entry.URL,
		Code:        entry.RedirectType,
	}

	if entry.Disabled {
		base.Status = StatusGone
		return base
	}
	if entry.ExpiresAt != nil && !entry.ExpiresAt.After(time.Now()) {
		base.Status = StatusGone
		return base
	}

	// Click limits are enforced with an atomic counter rather than the cached
	// count, which would be stale the moment a second request arrived.
	if entry.MaxClicks != nil {
		count, err := s.cache.IncrementClicks(ctx, entry.LinkID)
		if err != nil {
			// Fail open: a Redis hiccup should not take working links offline.
			s.logger.Warn("click limit check failed, allowing redirect",
				slog.String("link_id", entry.LinkID.String()),
				slog.String("error", err.Error()),
			)
		} else if count > *entry.MaxClicks {
			base.Status = StatusGone
			return base
		}
	}

	if entry.HasPassword {
		base.Status = StatusPassword
		base.NeedsAuth = true
		return base
	}

	if base.Code == 0 {
		base.Code = 302
	}
	base.Status = StatusRedirect
	return base
}

// loadFromDatabase handles a cache miss and writes the result back.
func (s *Service) loadFromDatabase(ctx context.Context, hostname, linkSlug string) *Cached {
	row, err := s.q.ResolveLink(ctx, store.ResolveLinkParams{
		Hostname: hostname,
		Slug:     linkSlug,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Negative caching: cheap protection against slug-scanning traffic.
			s.cache.Set(ctx, hostname, linkSlug, uuid.Nil, Cached{NotFound: true})
			return &Cached{NotFound: true}
		}
		s.logger.Error("resolve link from database",
			slog.String("hostname", hostname),
			slog.String("error", err.Error()),
		)
		return nil
	}

	entry := Cached{
		LinkID:       row.ID,
		WorkspaceID:  row.WorkspaceID,
		URL:          row.DestinationUrl,
		RedirectType: int(row.RedirectType),
		ExpiresAt:    row.ExpiresAt,
		MaxClicks:    row.MaxClicks,
		HasPassword:  row.PasswordHash != nil && *row.PasswordHash != "",
		Disabled:     row.Status != "active",
	}

	// The cache key is per host+slug, and the domain index needs the domain ID
	// to support whole-domain invalidation. ResolveLink does not return it, so
	// look it up once here on the (rare) miss path.
	if domain, err := s.q.GetDomainByHostname(ctx, hostname); err == nil {
		s.cache.Set(ctx, hostname, linkSlug, domain.ID, entry)
	}

	if entry.MaxClicks != nil {
		s.cache.SeedClickCounter(ctx, entry.LinkID, row.ClickCount)
	}

	return &entry
}

// VerifyPasswordFor checks a password supplied at the interstitial. It reads
// PostgreSQL because the hash is deliberately never cached.
func (s *Service) VerifyPasswordFor(ctx context.Context, linkID uuid.UUID, password string) (bool, error) {
	row, err := s.q.GetLink(ctx, linkID)
	if err != nil {
		return false, err
	}
	if row.PasswordHash == nil || *row.PasswordHash == "" {
		return true, nil
	}
	return verifyPassword(password, *row.PasswordHash)
}
