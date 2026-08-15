package link

import (
	"context"
	"fmt"
	"strings"

	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

// DefaultListLimit and MaxListLimit bound a page.
const (
	DefaultListLimit = 25
	MaxListLimit     = 100
)

// List returns a filtered page of links plus the cursor for the next page.
//
// This query is hand-written rather than generated: the filter combination is
// chosen at runtime, and expressing that in sqlc would mean either one query
// per combination or a tangle of `coalesce(sqlc.narg(...))` that PostgreSQL
// cannot plan well. Every value is still a bound parameter — nothing from the
// caller is interpolated into the SQL text.
func (s *Service) List(ctx context.Context, in ListInput) ([]View, *string, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}

	var (
		conds = []string{"links.workspace_id = $1"}
		args  = []any{in.WorkspaceID}
	)
	add := func(cond string, value any) {
		args = append(args, value)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}

	if in.Status != "" {
		if !isValidStatus(in.Status) {
			return nil, nil, httpx.Invalid(map[string][]string{
				"status": {"must be one of active, disabled, archived"},
			})
		}
		add("links.status = $%d", in.Status)
	}
	if in.DomainID != nil {
		add("links.domain_id = $%d", *in.DomainID)
	}
	if in.ExternalReference != "" {
		add("links.external_reference = $%d", in.ExternalReference)
	}
	if in.CreatedAfter != nil {
		add("links.created_at >= $%d", *in.CreatedAfter)
	}
	if in.CreatedBefore != nil {
		add("links.created_at <= $%d", *in.CreatedBefore)
	}
	if search := strings.TrimSpace(in.Search); search != "" {
		// ILIKE over slug, title, and destination. The GIN index backs the
		// full-text case; this stays a plain predicate so partial words match,
		// which is what a dashboard search box needs.
		args = append(args, "%"+search+"%")
		conds = append(conds, fmt.Sprintf(
			"(links.slug ILIKE $%d OR links.title ILIKE $%d OR links.destination_url ILIKE $%d)",
			len(args), len(args), len(args)))
	}

	if in.Cursor != "" {
		ts, id, err := decodeCursor(in.Cursor)
		if err != nil {
			return nil, nil, httpx.BadRequest("Invalid cursor")
		}
		args = append(args, ts, id)
		// Keyset comparison on the same (created_at DESC, id DESC) ordering
		// the index provides.
		conds = append(conds, fmt.Sprintf(
			"(links.created_at, links.id) < ($%d, $%d)", len(args)-1, len(args)))
	}

	// Fetch one extra row to learn whether another page exists without a
	// second COUNT query.
	args = append(args, limit+1)

	query := `
        SELECT
            links.id, links.workspace_id, links.domain_id, links.slug,
            links.destination_url, links.title, links.status, links.redirect_type,
            links.password_hash, links.expires_at, links.max_clicks, links.click_count,
            links.external_reference, links.metadata, links.created_by,
            links.created_via, links.created_at, links.updated_at,
            domains.hostname
        FROM links
        JOIN domains ON domains.id = links.domain_id
        WHERE ` + strings.Join(conds, " AND ") + `
        ORDER BY links.created_at DESC, links.id DESC
        LIMIT $` + fmt.Sprint(len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, httpx.Internal(err)
	}
	defer rows.Close()

	views := make([]View, 0, limit)
	for rows.Next() {
		var l store.Link
		var hostname string
		if err := rows.Scan(
			&l.ID, &l.WorkspaceID, &l.DomainID, &l.Slug,
			&l.DestinationUrl, &l.Title, &l.Status, &l.RedirectType,
			&l.PasswordHash, &l.ExpiresAt, &l.MaxClicks, &l.ClickCount,
			&l.ExternalReference, &l.Metadata, &l.CreatedBy,
			&l.CreatedVia, &l.CreatedAt, &l.UpdatedAt,
			&hostname,
		); err != nil {
			return nil, nil, httpx.Internal(err)
		}
		views = append(views, View{Link: l, Hostname: hostname})
	}
	if err := rows.Err(); err != nil {
		return nil, nil, httpx.Internal(err)
	}

	var nextCursor *string
	if len(views) > limit {
		views = views[:limit]
		last := views[len(views)-1]
		cursor := encodeCursor(last.Link.CreatedAt, last.Link.ID)
		nextCursor = &cursor
	}

	return views, nextCursor, nil
}
