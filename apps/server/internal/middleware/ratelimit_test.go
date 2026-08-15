package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimiterAllowsTheFullMinuteAsABurst(t *testing.T) {
	// "10 per minute" must mean ten immediately, not one every six seconds.
	// A user who mistypes a password should get their retry.
	rl := NewRateLimiter(10)

	for i := 1; i <= 10; i++ {
		if !rl.Allow("client") {
			t.Fatalf("request %d of 10 was refused within the per-minute allowance", i)
		}
	}
	if rl.Allow("client") {
		t.Error("request 11 was allowed; the limit is not enforced")
	}
}

func TestRateLimiterIsPerKey(t *testing.T) {
	rl := NewRateLimiter(2)

	for range 2 {
		rl.Allow("a")
	}
	if rl.Allow("a") {
		t.Fatal("key a exceeded its allowance")
	}
	if !rl.Allow("b") {
		t.Error("key b was refused because key a exhausted its own bucket")
	}
}

func TestRateLimiterMinimumBurst(t *testing.T) {
	rl := NewRateLimiter(0)
	if !rl.Allow("client") {
		t.Error("a zero limit must still admit one request rather than block everything")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := NewRateLimiter(1)
	handler := RateLimit(rl, ByIP)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	newReq := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "203.0.113.5:1234"
		return r
	}

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, newReq())
	if first.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", first.Code)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, newReq())
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Error("a 429 must carry Retry-After so a client knows when to try again")
	}
}
