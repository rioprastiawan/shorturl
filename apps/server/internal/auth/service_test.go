package auth

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rioprastiawan/shorturl/apps/server/internal/security"
)

// TestDummyPasswordHashIsUsable guards the login timing defence. A malformed
// constant would make VerifyPassword return ErrInvalidHash immediately instead
// of doing the Argon2 work, and the unknown-email path would answer measurably
// faster than a wrong-password one — turning login into an account oracle.
func TestDummyPasswordHashIsUsable(t *testing.T) {
	ok, err := security.VerifyPassword("anything at all", dummyPasswordHash)
	if err != nil {
		t.Fatalf("VerifyPassword(dummyPasswordHash) error = %v, want nil", err)
	}
	if ok {
		t.Error("VerifyPassword(dummyPasswordHash) = true, want false: the placeholder must not be a usable password")
	}
}

func TestInvalidCredentialsIsOpaque(t *testing.T) {
	if errInvalidCredentials.Status != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", errInvalidCredentials.Status, http.StatusUnauthorized)
	}
	// The message must not say whether the account exists.
	for _, word := range []string{"account", "user", "exist", "found", "registered"} {
		if strings.Contains(strings.ToLower(errInvalidCredentials.Message), word) {
			t.Errorf("Message = %q, must not hint at account existence (%q)", errInvalidCredentials.Message, word)
		}
	}
}

func TestOptional(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want *string
	}{
		{"empty becomes NULL", "", nil},
		{"value is kept", "Mozilla/5.0", ptr("Mozilla/5.0")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := optional(tt.in)
			switch {
			case tt.want == nil && got != nil:
				t.Errorf("optional(%q) = %q, want nil", tt.in, *got)
			case tt.want != nil && got == nil:
				t.Errorf("optional(%q) = nil, want %q", tt.in, *tt.want)
			case tt.want != nil && *got != *tt.want:
				t.Errorf("optional(%q) = %q, want %q", tt.in, *got, *tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"under the limit", "short", 10, "short"},
		{"at the limit", "exactlyten", 10, "exactlyten"},
		{"over the limit", "far too long to keep", 10, "far too lo"},
		{"empty", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncate(tt.in, tt.max); got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
		})
	}
}

func ptr(s string) *string { return &s }
