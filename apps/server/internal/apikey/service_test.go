package apikey

import (
	"strings"
	"testing"
	"time"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/security"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

func TestNewKeyFormat(t *testing.T) {
	for _, environment := range []string{LivePrefix, TestPrefix} {
		t.Run(environment, func(t *testing.T) {
			plaintext, prefix, err := newKey(environment)
			if err != nil {
				t.Fatalf("newKey: %v", err)
			}

			if !strings.HasPrefix(plaintext, environment) {
				t.Errorf("plaintext %q does not start with %q", plaintext, environment)
			}
			secret := strings.TrimPrefix(plaintext, environment)
			if len(secret) != SecretLength {
				t.Errorf("secret length = %d, want %d", len(secret), SecretLength)
			}
			if want := environment + secret[:prefixSampleLen]; prefix != want {
				t.Errorf("prefix = %q, want %q", prefix, want)
			}
			if len(prefix) > 32 {
				t.Errorf("prefix %q exceeds the key_prefix column width", prefix)
			}

			// The prefix must be what a later Authenticate derives from the
			// token, or the lookup can never find the row.
			got, ok := ParsePrefix(plaintext)
			if !ok || got != prefix {
				t.Errorf("ParsePrefix(%q) = %q, %v; want %q, true", plaintext, got, ok, prefix)
			}
		})
	}
}

func TestNewKeyIsUnique(t *testing.T) {
	seen := make(map[string]bool, 64)
	for range 64 {
		plaintext, _, err := newKey(LivePrefix)
		if err != nil {
			t.Fatalf("newKey: %v", err)
		}
		if seen[plaintext] {
			t.Fatalf("newKey returned a duplicate token")
		}
		seen[plaintext] = true
	}
}

func TestParsePrefix(t *testing.T) {
	valid := LivePrefix + strings.Repeat("a", SecretLength)

	tests := []struct {
		name   string
		token  string
		want   string
		wantOK bool
	}{
		{"live key", valid, LivePrefix + "aaaaaaaa", true},
		{"test key", TestPrefix + strings.Repeat("b", SecretLength), TestPrefix + "bbbbbbbb", true},
		{"empty", "", "", false},
		{"no prefix", strings.Repeat("a", SecretLength), "", false},
		{"unknown prefix", "shr_dev_" + strings.Repeat("a", SecretLength), "", false},
		{"secret too short", LivePrefix + "abc", "", false},
		{"secret too long", valid + "a", "", false},
		{"prefix only", LivePrefix, "", false},
		{"session token shape", "sess_" + strings.Repeat("a", 40), "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParsePrefix(tc.token)
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("ParsePrefix(%q) = %q, %v; want %q, %v", tc.token, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestUsable(t *testing.T) {
	plaintext := LivePrefix + strings.Repeat("a", SecretLength)
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	live := func() store.ApiKey {
		return store.ApiKey{KeyHash: security.HashToken(plaintext)}
	}

	tests := []struct {
		name   string
		row    store.ApiKey
		bearer string
		want   bool
	}{
		{
			name:   "valid key",
			row:    live(),
			bearer: plaintext,
			want:   true,
		},
		{
			name:   "wrong key with the same prefix",
			row:    live(),
			bearer: LivePrefix + strings.Repeat("b", SecretLength),
			want:   false,
		},
		{
			name: "revoked",
			row: func() store.ApiKey {
				k := live()
				k.RevokedAt = &past
				return k
			}(),
			bearer: plaintext,
			want:   false,
		},
		{
			name: "expired",
			row: func() store.ApiKey {
				k := live()
				k.ExpiresAt = &past
				return k
			}(),
			bearer: plaintext,
			want:   false,
		},
		{
			name: "expiring in the future",
			row: func() store.ApiKey {
				k := live()
				k.ExpiresAt = &future
				return k
			}(),
			bearer: plaintext,
			want:   true,
		},
		{
			name: "expires exactly now",
			row: func() store.ApiKey {
				k := live()
				k.ExpiresAt = &now
				return k
			}(),
			bearer: plaintext,
			want:   false,
		},
		{
			name: "revoked beats a valid expiry",
			row: func() store.ApiKey {
				k := live()
				k.RevokedAt = &past
				k.ExpiresAt = &future
				return k
			}(),
			bearer: plaintext,
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := usable(tc.row, tc.bearer, now); got != tc.want {
				t.Errorf("usable() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNormalizeScopes(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil defaults", nil, DefaultScopes},
		{"empty defaults", []string{}, DefaultScopes},
		{"blank only defaults", []string{" ", ""}, DefaultScopes},
		{"deduplicates", []string{authctx.ScopeLinksRead, authctx.ScopeLinksRead}, []string{authctx.ScopeLinksRead}},
		{"trims", []string{" links:read "}, []string{authctx.ScopeLinksRead}},
		{"keeps order", []string{authctx.ScopeAnalyticsRead, authctx.ScopeLinksRead},
			[]string{authctx.ScopeAnalyticsRead, authctx.ScopeLinksRead}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeScopes(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("normalizeScopes(%v) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("normalizeScopes(%v) = %v, want %v", tc.in, got, tc.want)
				}
			}
		})
	}
}
