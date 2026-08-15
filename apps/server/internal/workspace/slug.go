package workspace

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

// MaxSlugLength matches workspaces.slug VARCHAR(64).
const MaxSlugLength = 64

// fallbackSlug is used when a name has no alphanumeric characters at all, for
// example a workspace named entirely in emoji.
const fallbackSlug = "workspace"

// maxSlugAttempts bounds the numeric-suffix search. Fifty distinct workspaces
// sharing one name is already pathological; beyond that the caller is told to
// pick another name rather than the server looping.
const maxSlugAttempts = 50

// Slugify derives a URL-safe slug from a display name: lowercased, every run
// of non-alphanumeric characters collapsed to a single hyphen, trimmed, and
// capped at the column width.
//
// Only ASCII letters and digits survive, matching the character set
// internal/slug accepts for links, so a slug is always safe to put in a URL
// without percent-encoding. A name with nothing in that set — written entirely
// in a non-Latin script, say — falls back to a generated slug, which is why
// the workspace name is what the interface shows.
func Slugify(name string) string {
	var b strings.Builder
	b.Grow(len(name))

	pendingSeparator := false
	for _, r := range strings.ToLower(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			// Only emit a separator once something follows it, which collapses
			// runs and drops leading ones without a second pass.
			if pendingSeparator && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingSeparator = false
			b.WriteRune(r)
		default:
			pendingSeparator = true
		}
	}

	slug := truncateSlug(b.String(), MaxSlugLength)
	if slug == "" {
		return fallbackSlug
	}
	return slug
}

// slugAttempt returns the nth candidate for a base slug: the base itself for
// n == 1, then base-2, base-3, and so on, always within the column width.
func slugAttempt(base string, n int) string {
	if n <= 1 {
		return base
	}
	suffix := "-" + strconv.Itoa(n)
	return truncateSlug(base, MaxSlugLength-len(suffix)) + suffix
}

// truncateSlug cuts to a byte budget without leaving a stray hyphen at either
// end. Slugify has already reduced the string to ASCII, so a byte cut cannot
// split a rune.
func truncateSlug(s string, max int) string {
	if max < 1 {
		max = 1
	}
	if len(s) > max {
		s = s[:max]
	}
	return strings.Trim(s, "-")
}

// UniqueSlug derives a slug from name and probes for one that is free. It
// takes a *store.Queries so it can run either on the pool or inside a
// transaction; setup uses the transactional form.
//
// This is a check-then-insert, so a concurrent create can still lose the race
// on the unique index. Callers map that 23505 to a conflict rather than
// retrying, because a failed statement has already aborted the transaction.
func UniqueSlug(ctx context.Context, q *store.Queries, name string) (string, error) {
	base := Slugify(name)

	for n := 1; n <= maxSlugAttempts; n++ {
		candidate := slugAttempt(base, n)
		_, err := q.GetWorkspaceBySlug(ctx, candidate)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return candidate, nil
		case err != nil:
			return "", httpx.Internal(err)
		}
	}

	return "", httpx.Conflictf("slug_taken", "Too many workspaces share that name; please choose another")
}
