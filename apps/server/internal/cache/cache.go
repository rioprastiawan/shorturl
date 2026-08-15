// Package cache owns the Redis connection and the key layout.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps the Redis client with the project's key conventions.
type Client struct {
	rdb *redis.Client
}

// Connect opens a Redis client and verifies it responds before returning.
func Connect(ctx context.Context, addr, password string) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:            addr,
		Password:        password,
		DB:              0,
		PoolSize:        20,
		MinIdleConns:    2,
		ConnMaxIdleTime: 5 * time.Minute,
		// The redirect path must fail fast rather than hang: a slow Redis
		// falls back to PostgreSQL, a hanging one would stall the request.
		DialTimeout:  3 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Redis exposes the underlying client for the stream producer and consumer.
func (c *Client) Redis() *redis.Client { return c.rdb }

// Ping reports whether Redis is reachable, for the readiness probe.
func (c *Client) Ping(ctx context.Context) error { return c.rdb.Ping(ctx).Err() }

// Close releases the connection pool.
func (c *Client) Close() error { return c.rdb.Close() }

// LinkKey is the cache key for a resolved redirect target.
func LinkKey(hostname, slug string) string {
	return "shorturl:link:" + hostname + ":" + slug
}

// LinkDomainSetKey indexes every cached link key belonging to one domain, so
// disabling a domain can invalidate all of its links without a SCAN.
func LinkDomainSetKey(domainID string) string {
	return "shorturl:domain-links:" + domainID
}
