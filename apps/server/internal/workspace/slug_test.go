package workspace

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases", "Acme", "acme"},
		{"spaces become hyphens", "Acme Corp", "acme-corp"},
		{"collapses runs", "Acme   ///  Corp", "acme-corp"},
		{"trims leading and trailing separators", "  --Acme Corp--  ", "acme-corp"},
		{"strips punctuation", "Acme, Inc. (2024)!", "acme-inc-2024"},
		{"keeps digits", "Team 42", "team-42"},
		{"underscores are separators", "acme_corp", "acme-corp"},
		{"already a slug", "acme-corp", "acme-corp"},
		{"empty falls back", "", fallbackSlug},
		{"whitespace only falls back", "   \t\n ", fallbackSlug},
		{"punctuation only falls back", "!!! ---", fallbackSlug},
		{"emoji only falls back", "🚀🚀", fallbackSlug},
		{"non-latin script falls back", "北京团队", fallbackSlug},
		{"accents are separators, not letters", "Zürich Team", "z-rich-team"},
		{"single character", "X", "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Slugify(tt.in); got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSlugifyLength(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"long ascii", strings.Repeat("a", 200)},
		{"long with separators", strings.Repeat("ab ", 100)},
		// Every rune becomes a separator, so this must not truncate to a
		// hyphen-only string.
		{"long unicode", strings.Repeat("é", 200)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.in)
			if len(got) > MaxSlugLength {
				t.Errorf("Slugify(...) = %d bytes, want at most %d", len(got), MaxSlugLength)
			}
			if strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") {
				t.Errorf("Slugify(...) = %q, want no leading or trailing hyphen", got)
			}
		})
	}
}

func TestSlugAttempt(t *testing.T) {
	tests := []struct {
		name string
		base string
		n    int
		want string
	}{
		{"first attempt is the base", "acme", 1, "acme"},
		{"zero is treated as first", "acme", 0, "acme"},
		{"second appends 2", "acme", 2, "acme-2"},
		{"fiftieth appends 50", "acme", 50, "acme-50"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := slugAttempt(tt.base, tt.n); got != tt.want {
				t.Errorf("slugAttempt(%q, %d) = %q, want %q", tt.base, tt.n, got, tt.want)
			}
		})
	}
}

func TestSlugAttemptFitsColumn(t *testing.T) {
	base := Slugify(strings.Repeat("a", 200))

	for n := 1; n <= maxSlugAttempts; n++ {
		got := slugAttempt(base, n)
		if len(got) > MaxSlugLength {
			t.Fatalf("slugAttempt(base, %d) = %d bytes, want at most %d", n, len(got), MaxSlugLength)
		}
		if strings.Contains(got, "--") {
			t.Fatalf("slugAttempt(base, %d) = %q, want no doubled hyphen", n, got)
		}
	}
}

func TestSlugAttemptsAreDistinct(t *testing.T) {
	base := Slugify(strings.Repeat("a", 200))
	seen := make(map[string]int, maxSlugAttempts)

	for n := 1; n <= maxSlugAttempts; n++ {
		got := slugAttempt(base, n)
		if prev, ok := seen[got]; ok {
			t.Fatalf("attempt %d produced %q, already produced by attempt %d", n, got, prev)
		}
		seen[got] = n
	}
}
