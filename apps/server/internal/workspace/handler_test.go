package workspace

import (
	"strings"
	"testing"

	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		want      string
		wantField string
	}{
		{"plain name", "Acme", "Acme", ""},
		{"trims surrounding space", "  Acme Corp  ", "Acme Corp", ""},
		{"minimum length", "Ax", "Ax", ""},
		{"maximum length", strings.Repeat("a", NameMaxLength), strings.Repeat("a", NameMaxLength), ""},
		{"empty", "", "", "name"},
		{"whitespace only", "   ", "", "name"},
		{"too short", "A", "", "name"},
		{"too long", strings.Repeat("a", NameMaxLength+1), "", "name"},
		// Rune count, not byte count: a 3-character name in a multi-byte
		// script must not be rejected as too long.
		{"multi-byte characters count as runes", "北京团队", "北京团队", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateName(tt.in)
			if got != tt.want {
				t.Errorf("validateName(%q) = %q, want %q", tt.in, got, tt.want)
			}

			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("validateName(%q) error = %v, want nil", tt.in, err)
				}
				return
			}

			if err == nil {
				t.Fatalf("validateName(%q) error = nil, want a validation error", tt.in)
			}
			if fields := httpx.AsAPIError(err).Fields; len(fields[tt.wantField]) == 0 {
				t.Errorf("validateName(%q) fields = %v, want an error on %q", tt.in, fields, tt.wantField)
			}
		})
	}
}
