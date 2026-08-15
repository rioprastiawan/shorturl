package slug

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	seen := make(map[string]bool, 500)
	for range 500 {
		s, err := Generate(DefaultLength)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if len(s) != DefaultLength {
			t.Fatalf("length = %d, want %d", len(s), DefaultLength)
		}
		if seen[s] {
			t.Fatalf("Generate() collided on %q within 500 calls", s)
		}
		seen[s] = true

		for _, r := range s {
			if !strings.ContainsRune(Alphabet, r) {
				t.Fatalf("slug %q contains %q, which is outside the alphabet", s, r)
			}
		}
	}
}

func TestAlphabetExcludesAmbiguousCharacters(t *testing.T) {
	// These are the pairs people misread when a link is printed or dictated.
	for _, r := range "0O1lI" {
		if strings.ContainsRune(Alphabet, r) {
			t.Errorf("alphabet contains the ambiguous character %q", r)
		}
	}
}

func TestGenerateDefaultsLength(t *testing.T) {
	s, err := Generate(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != DefaultLength {
		t.Errorf("Generate(0) length = %d, want the default %d", len(s), DefaultLength)
	}
}

func TestValidate(t *testing.T) {
	valid := []string{"promo", "invoice-12345", "a", "A_b-9", strings.Repeat("x", MaxLength)}
	for _, s := range valid {
		if err := Validate(s); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", truncate(s), err)
		}
	}

	invalid := map[string]string{
		"empty":         "",
		"too long":      strings.Repeat("x", MaxLength+1),
		"slash":         "a/b",
		"dot":           "file.txt",
		"space":         "hello world",
		"question mark": "a?b",
		"percent":       "a%20b",
		"unicode":       "héllo",
		"hash":          "a#b",
	}
	for name, s := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := Validate(s); err == nil {
				t.Errorf("Validate(%q) = nil, want an error", truncate(s))
			}
		})
	}
}

func TestValidateRejectsReservedPaths(t *testing.T) {
	// A link on one of these would shadow the dashboard or the API when the
	// main application domain is also used for short links.
	for _, s := range []string{"api", "API", "login", "dashboard", "health", "_nuxt", "robots.txt"} {
		t.Run(s, func(t *testing.T) {
			if err := Validate(s); err == nil {
				t.Errorf("Validate(%q) = nil, want a reserved-path error", s)
			}
			if !IsReserved(s) {
				t.Errorf("IsReserved(%q) = false, want true", s)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"/promo":    "promo",
		"promo/":    "promo",
		"/promo/":   "promo",
		"  promo  ": "promo",
		"/":         "",
		"":          "",
		"a/b":       "a/b", // inner slashes survive; the caller rejects them
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func truncate(s string) string {
	if len(s) > 20 {
		return s[:20] + "..."
	}
	return s
}
