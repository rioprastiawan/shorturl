package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
)

// RateLimiter is an in-memory token-bucket limiter keyed by an arbitrary
// string (client IP, API key ID, ...).
//
// In-memory is the right choice for a single-node self-hosted deployment: it
// costs no Redis round-trip on the hot path. If the stack is ever scaled to
// several server containers, each would enforce its own share of the limit —
// see docs/deployment.md before doing that.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	limit    rate.Limit
	burst    int
	lastGC   time.Time
	gcPeriod time.Duration
}

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter allows perMinute requests per key.
//
// The burst equals the full per-minute allowance, which is what "10 requests
// per minute" is normally taken to mean: ten now, then refilling steadily. A
// smaller burst would spread the allowance evenly instead, so at 10/min a user
// who mistypes their password once would be refused the immediate retry —
// technically within the limit, but broken from the user's side.
func NewRateLimiter(perMinute int) *RateLimiter {
	// Config validation already enforces a floor, but a constructor that
	// divides by its argument must not panic on a bad one.
	if perMinute < 1 {
		perMinute = 1
	}
	return &RateLimiter{
		buckets:  make(map[string]*bucket),
		limit:    rate.Every(time.Minute / time.Duration(perMinute)),
		burst:    perMinute,
		lastGC:   time.Now(),
		gcPeriod: 10 * time.Minute,
	}
}

// Allow reports whether the key may proceed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.Sub(rl.lastGC) > rl.gcPeriod {
		rl.collect(now)
		rl.lastGC = now
	}

	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.buckets[key] = b
	}
	b.lastSeen = now
	return b.limiter.Allow()
}

// collect drops buckets untouched for a while, so the map cannot grow without
// bound under a spray of distinct IPs.
func (rl *RateLimiter) collect(now time.Time) {
	for key, b := range rl.buckets {
		if now.Sub(b.lastSeen) > rl.gcPeriod {
			delete(rl.buckets, key)
		}
	}
}

// KeyFunc derives the rate-limit key for a request.
type KeyFunc func(*http.Request) string

// RateLimit rejects requests over the limit with a 429 and a Retry-After hint.
func RateLimit(rl *RateLimiter, key KeyFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.Allow(key(r)) {
				w.Header().Set("Retry-After", strconv.Itoa(60))
				httpx.Error(w, r, httpx.ErrRateLimited)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ByIP keys the limiter on the resolved client address.
func ByIP(r *http.Request) string { return ClientIP(r) }
