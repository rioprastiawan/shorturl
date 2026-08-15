package link

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/rioprastiawan/shorturl/apps/server/internal/cache"
)

// Cached is the redirect payload. Field names are single letters because this
// value is serialised on every cache write and read on every redirect.
type Cached struct {
	LinkID       uuid.UUID  `json:"l"`
	WorkspaceID  uuid.UUID  `json:"w"`
	URL          string     `json:"u"`
	RedirectType int        `json:"r"`
	ExpiresAt    *time.Time `json:"e,omitempty"`
	MaxClicks    *int64     `json:"m,omitempty"`
	HasPassword  bool       `json:"p,omitempty"`
	Disabled     bool       `json:"d,omitempty"`

	// NotFound marks a negative cache entry. Without it, traffic probing for
	// random slugs would reach PostgreSQL on every request.
	NotFound bool `json:"n,omitempty"`
}

// negativeTTL is deliberately short: a slug that 404s now is often created a
// moment later, and a user who just made a link should not wait an hour.
const negativeTTL = 30 * time.Second

// clickCounterKey tracks click-limit enforcement in Redis.
//
// The cached payload cannot carry a live click count, and reading the count
// from PostgreSQL on every redirect would defeat the cache. An atomic INCR is
// the only way to enforce max_clicks correctly without a database round-trip.
func clickCounterKey(linkID uuid.UUID) string {
	return "shorturl:clicks:" + linkID.String()
}

// Cache wraps the Redis operations the redirect path and the link service
// share.
type Cache struct {
	rdb    *redis.Client
	ttl    time.Duration
	logger *slog.Logger
}

// NewCache builds the cache helper.
func NewCache(c *cache.Client, ttl time.Duration, logger *slog.Logger) *Cache {
	return &Cache{rdb: c.Redis(), ttl: ttl, logger: logger}
}

// Get reads a cached entry. A miss returns (nil, nil) so callers distinguish
// "not cached" from "cached as missing".
func (c *Cache) Get(ctx context.Context, hostname, slug string) (*Cached, error) {
	raw, err := c.rdb.Get(ctx, cache.LinkKey(hostname, slug)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	var entry Cached
	if err := json.Unmarshal(raw, &entry); err != nil {
		// A corrupt entry should not break redirects; treat it as a miss.
		return nil, nil
	}
	return &entry, nil
}

// Set stores an entry and records it in the domain's index so the whole domain
// can be invalidated later without a SCAN over the keyspace.
func (c *Cache) Set(ctx context.Context, hostname, slug string, domainID uuid.UUID, entry Cached) {
	payload, err := json.Marshal(entry)
	if err != nil {
		c.logger.Warn("marshal cache entry", slog.String("error", err.Error()))
		return
	}

	ttl := c.ttl
	if entry.NotFound {
		ttl = negativeTTL
	}

	key := cache.LinkKey(hostname, slug)
	pipe := c.rdb.TxPipeline()
	pipe.Set(ctx, key, payload, ttl)
	if !entry.NotFound {
		indexKey := cache.LinkDomainSetKey(domainID.String())
		pipe.SAdd(ctx, indexKey, key)
		// The index outlives individual entries but must not leak forever if a
		// domain is deleted without going through the service.
		pipe.Expire(ctx, indexKey, 7*24*time.Hour)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		c.logger.Warn("write cache entry", slog.String("error", err.Error()))
	}
}

// SeedClickCounter primes the click-limit counter from the authoritative
// database value. Called on a cache miss for a link that has a limit.
func (c *Cache) SeedClickCounter(ctx context.Context, linkID uuid.UUID, count int64) {
	// SetNX: never overwrite a counter that a concurrent redirect already
	// advanced past the database value.
	if err := c.rdb.SetNX(ctx, clickCounterKey(linkID), count, 0).Err(); err != nil {
		c.logger.Warn("seed click counter", slog.String("error", err.Error()))
	}
}

// IncrementClicks atomically bumps and returns the click count for a limited
// link.
func (c *Cache) IncrementClicks(ctx context.Context, linkID uuid.UUID) (int64, error) {
	return c.rdb.Incr(ctx, clickCounterKey(linkID)).Result()
}

// Invalidate drops one link's entry. Called on every mutation, because
// correctness must not depend on the TTL expiring.
func (c *Cache) Invalidate(ctx context.Context, hostname, slug string) {
	if err := c.rdb.Del(ctx, cache.LinkKey(hostname, slug)).Err(); err != nil {
		c.logger.Warn("invalidate cache entry",
			slog.String("hostname", hostname),
			slog.String("slug", slug),
			slog.String("error", err.Error()),
		)
	}
}

// InvalidateLink drops the entry and the click counter, used when a link is
// deleted outright.
func (c *Cache) InvalidateLink(ctx context.Context, hostname, slug string, linkID uuid.UUID) {
	if err := c.rdb.Del(ctx, cache.LinkKey(hostname, slug), clickCounterKey(linkID)).Err(); err != nil {
		c.logger.Warn("invalidate link", slog.String("error", err.Error()))
	}
}

// InvalidateDomain drops every cached link for a domain, for when a domain is
// disabled or deleted.
func (c *Cache) InvalidateDomain(ctx context.Context, domainID uuid.UUID) {
	indexKey := cache.LinkDomainSetKey(domainID.String())
	keys, err := c.rdb.SMembers(ctx, indexKey).Result()
	if err != nil {
		c.logger.Warn("read domain link index", slog.String("error", err.Error()))
		return
	}
	if len(keys) > 0 {
		if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
			c.logger.Warn("invalidate domain links", slog.String("error", err.Error()))
		}
	}
	if err := c.rdb.Del(ctx, indexKey).Err(); err != nil {
		c.logger.Warn("drop domain link index", slog.String("error", err.Error()))
	}
}
