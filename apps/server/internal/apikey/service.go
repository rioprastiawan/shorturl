// Package apikey issues and authenticates the bearer tokens that machine
// callers use against the public REST API (plan §13).
//
// A key is shown to its creator exactly once. Only a lookup prefix and a
// SHA-256 digest are stored, so a database leak yields nothing usable.
package apikey

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/security"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
	"github.com/rioprastiawan/shorturl/apps/server/internal/validate"
)

// Token shape. The environment marker is part of the secret so an integration
// can tell staging from production traffic in its own logs without holding the
// key: only the prefix ever appears in ours.
const (
	LivePrefix = "shr_live_"
	TestPrefix = "shr_test_"

	// secretBytes yields exactly 40 URL-safe characters through base64 raw
	// encoding, which is what the documented key format promises.
	secretBytes = 30
	// SecretLength is the character count of the random half of a key.
	SecretLength = 40
	// prefixSampleLen is how much of the secret goes into the stored prefix.
	// Eight characters of base64 is 48 bits — enough that a collision is a
	// curiosity rather than a design problem, and far too little to help an
	// attacker guess the remaining 32.
	prefixSampleLen = 8
)

// MaxNameLength matches the api_keys.name column.
const MaxNameLength = 120

// createAttempts bounds prefix regeneration when the unique index rejects a
// candidate. Needing a second attempt is already vanishingly unlikely.
const createAttempts = 5

// touchInterval is the floor between two last_used_at writes for one key.
// Without it every API call would carry an UPDATE, which plan §13 explicitly
// rules out.
const touchInterval = time.Minute

// touchTimeout bounds the detached last_used_at write so a stalled connection
// cannot leak goroutines.
const touchTimeout = 5 * time.Second

// Service issues, lists, revokes, and authenticates API keys.
type Service struct {
	pool   *pgxpool.Pool
	q      *store.Queries
	logger *slog.Logger

	// touched remembers when each key's last_used_at was last written. It is
	// bounded by the number of keys in the installation, which is small.
	mu      sync.Mutex
	touched map[uuid.UUID]time.Time
}

// NewService builds the API key service.
func NewService(pool *pgxpool.Pool, q *store.Queries, logger *slog.Logger) *Service {
	return &Service{pool: pool, q: q, logger: logger, touched: make(map[uuid.UUID]time.Time)}
}

// DefaultScopes is what a machine-to-machine key gets when the caller does not
// choose (plan §13): read and write links, but no analytics.
var DefaultScopes = []string{authctx.ScopeLinksRead, authctx.ScopeLinksWrite}

// Create issues a key and returns the stored row alongside the plaintext.
//
// The plaintext is the only copy that will ever exist; the caller must hand it
// to the user immediately and keep no record of it.
func (s *Service) Create(
	ctx context.Context,
	workspaceID uuid.UUID,
	createdBy *uuid.UUID,
	name string,
	scopes []string,
	expiresAt *time.Time,
	test bool,
) (store.ApiKey, string, error) {
	v := validate.New()

	name = v.Required("name", name)
	if name != "" {
		v.Length("name", name, 1, MaxNameLength)
	}

	scopes = normalizeScopes(scopes)
	for _, scope := range scopes {
		v.Check(slices.Contains(authctx.ValidScopes, scope), "scopes",
			"%q is not a valid scope; must be one of %s", scope, strings.Join(authctx.ValidScopes, ", "))
	}

	if expiresAt != nil && !expiresAt.After(time.Now()) {
		v.Add("expires_at", "must be in the future")
	}

	if v.HasErrors() {
		return store.ApiKey{}, "", httpx.Invalid(v.Fields())
	}

	environment := LivePrefix
	if test {
		environment = TestPrefix
	}

	var lastErr error
	for range createAttempts {
		plaintext, prefix, err := newKey(environment)
		if err != nil {
			return store.ApiKey{}, "", httpx.Internal(err)
		}

		row, err := s.q.CreateAPIKey(ctx, store.CreateAPIKeyParams{
			WorkspaceID: workspaceID,
			Name:        name,
			KeyPrefix:   prefix,
			KeyHash:     security.HashToken(plaintext),
			Scopes:      scopes,
			ExpiresAt:   expiresAt,
			CreatedBy:   createdBy,
		})
		if err == nil {
			return row, plaintext, nil
		}
		if !isUniqueViolation(err) {
			return store.ApiKey{}, "", httpx.Internal(err)
		}
		lastErr = err
	}
	return store.ApiKey{}, "", httpx.Internal(lastErr)
}

// List returns a stable keyset-paginated page, including revoked keys.
func (s *Service) List(ctx context.Context, workspaceID uuid.UUID, cursor string, limit int) ([]store.ApiKey, *string, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	args := []any{workspaceID}
	condition := ""
	if cursor != "" {
		createdAt, id, err := decodeListCursor(cursor)
		if err != nil {
			return nil, nil, httpx.BadRequest("Invalid cursor")
		}
		args = append(args, createdAt, id)
		condition = " AND (created_at, id) < ($2, $3)"
	}
	args = append(args, limit+1)
	query := `SELECT id, workspace_id, name, key_prefix, key_hash, scopes,
        last_used_at, expires_at, revoked_at, created_by, created_at
        FROM api_keys WHERE workspace_id = $1` + condition + `
        ORDER BY created_at DESC, id DESC LIMIT $` + fmt.Sprint(len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, httpx.Internal(err)
	}
	defer rows.Close()
	items := make([]store.ApiKey, 0, limit+1)
	for rows.Next() {
		var item store.ApiKey
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.Name, &item.KeyPrefix,
			&item.KeyHash, &item.Scopes, &item.LastUsedAt, &item.ExpiresAt,
			&item.RevokedAt, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, nil, httpx.Internal(err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, httpx.Internal(err)
	}

	var next *string
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		value := base64.RawURLEncoding.EncodeToString([]byte(last.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + last.ID.String()))
		next = &value
	}
	return items, next, nil
}

func decodeListCursor(raw string) (time.Time, uuid.UUID, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	before, after, found := strings.Cut(string(decoded), "|")
	if !found {
		return time.Time{}, uuid.Nil, fmt.Errorf("malformed cursor")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, before)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	id, err := uuid.Parse(after)
	return createdAt, id, err
}

// Revoke marks a key unusable. Already-revoked and unknown keys look the same
// from here, because the query only matches rows that are still live.
func (s *Service) Revoke(ctx context.Context, workspaceID, keyID uuid.UUID) error {
	affected, err := s.q.RevokeAPIKey(ctx, store.RevokeAPIKeyParams{ID: keyID, WorkspaceID: workspaceID})
	if err != nil {
		return httpx.Internal(err)
	}
	if affected == 0 {
		return httpx.ErrNotFound
	}
	return nil
}

// Authenticate resolves a bearer token to its principal.
//
// Every rejection returns the same error: telling a caller that a key exists
// but is revoked, versus that it never existed, is free reconnaissance.
//
// Nothing here is cached. A revoked key must stop working on the next request,
// and a cache measured in seconds would be a window in which it still does.
func (s *Service) Authenticate(ctx context.Context, bearer string) (authctx.APIKeyPrincipal, error) {
	prefix, ok := ParsePrefix(bearer)
	if !ok {
		return authctx.APIKeyPrincipal{}, httpx.ErrUnauthorized
	}

	row, err := s.q.GetAPIKeyByPrefix(ctx, prefix)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authctx.APIKeyPrincipal{}, httpx.ErrUnauthorized
		}
		return authctx.APIKeyPrincipal{}, httpx.Internal(err)
	}

	if !usable(row, bearer, time.Now()) {
		return authctx.APIKeyPrincipal{}, httpx.ErrUnauthorized
	}

	s.touch(ctx, row.ID)
	return authctx.APIKeyPrincipal{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		Scopes:      row.Scopes,
	}, nil
}

// usable is the whole accept/reject decision for a looked-up key, kept pure so
// it can be tested without a database.
func usable(row store.ApiKey, bearer string, now time.Time) bool {
	// Constant time: the digest is derived from attacker-controlled input, so a
	// byte-by-byte early exit would leak how much of a guess was correct.
	if subtle.ConstantTimeCompare([]byte(security.HashToken(bearer)), []byte(row.KeyHash)) != 1 {
		return false
	}
	if row.RevokedAt != nil {
		return false
	}
	if row.ExpiresAt != nil && !row.ExpiresAt.After(now) {
		return false
	}
	return true
}

// ParsePrefix extracts the database lookup prefix from a bearer token, and
// reports whether the token is even shaped like one of ours. A malformed token
// never reaches the database.
func ParsePrefix(token string) (string, bool) {
	var environment string
	switch {
	case strings.HasPrefix(token, LivePrefix):
		environment = LivePrefix
	case strings.HasPrefix(token, TestPrefix):
		environment = TestPrefix
	default:
		return "", false
	}

	secret := token[len(environment):]
	if len(secret) != SecretLength {
		return "", false
	}
	return environment + secret[:prefixSampleLen], true
}

// newKey mints a token and its stored prefix.
func newKey(environment string) (plaintext, prefix string, err error) {
	secret, err := security.NewToken(secretBytes)
	if err != nil {
		return "", "", err
	}
	return environment + secret, environment + secret[:prefixSampleLen], nil
}

// touch records that a key was used, at most once a minute per key and never
// on the request's own goroutine.
func (s *Service) touch(ctx context.Context, id uuid.UUID) {
	now := time.Now()

	s.mu.Lock()
	if last, seen := s.touched[id]; seen && now.Sub(last) < touchInterval {
		s.mu.Unlock()
		return
	}
	s.touched[id] = now
	s.mu.Unlock()

	// WithoutCancel: the response is written and the request context cancelled
	// long before this UPDATE would finish, and an aborted write here would
	// leave last_used_at permanently stale.
	detached := context.WithoutCancel(ctx)
	go func() {
		writeCtx, cancel := context.WithTimeout(detached, touchTimeout)
		defer cancel()

		if err := s.q.TouchAPIKey(writeCtx, id); err != nil {
			// Bookkeeping only — never worth failing a request over.
			s.logger.Warn("recording api key use",
				slog.String("api_key_id", id.String()),
				slog.String("error", err.Error()),
			)
		}
	}()
}

// normalizeScopes applies the default set and removes duplicates so a caller
// sending links:read twice does not store it twice.
func normalizeScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return slices.Clone(DefaultScopes)
	}

	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" && !slices.Contains(out, scope) {
			out = append(out, scope)
		}
	}
	if len(out) == 0 {
		return slices.Clone(DefaultScopes)
	}
	return out
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
