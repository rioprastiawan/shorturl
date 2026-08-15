// Package slug generates and validates the short path of a link.
package slug

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// Alphabet excludes characters that are easy to confuse when a short link is
// read aloud or copied from print: 0/O, 1/l/I.
const Alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// DefaultLength gives 56^7 ≈ 1.7e12 possibilities, so random collisions stay
// negligible well past millions of links.
const DefaultLength = 7

// MaxLength bounds a caller-supplied slug.
const MaxLength = 128

// reserved paths must never become slugs, because the application serves them
// on the main domain and a link would shadow the dashboard or API.
var reserved = map[string]bool{
	"api": true, "admin": true, "login": true, "logout": true, "register": true,
	"health": true, "ready": true, "metrics": true, "setup": true, "dashboard": true,
	"docs": true, "static": true, "assets": true, "_nuxt": true,
	"favicon.ico": true, "robots.txt": true, "sitemap.xml": true,
	".well-known": true,
}

// Generate returns a cryptographically random slug.
func Generate(length int) (string, error) {
	if length <= 0 {
		length = DefaultLength
	}
	max := big.NewInt(int64(len(Alphabet)))

	var b strings.Builder
	b.Grow(length)
	for range length {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate slug: %w", err)
		}
		b.WriteByte(Alphabet[n.Int64()])
	}
	return b.String(), nil
}

// IsReserved reports whether a slug collides with an application route.
func IsReserved(s string) bool {
	return reserved[strings.ToLower(s)]
}

// Validate checks a caller-supplied slug.
//
// Allowed characters are letters, digits, hyphen, and underscore. Slashes and
// dots are rejected: a slug containing them would not round-trip through the
// single path segment the redirect handler matches.
func Validate(s string) error {
	switch {
	case s == "":
		return fmt.Errorf("must not be empty")
	case len(s) > MaxLength:
		return fmt.Errorf("must be at most %d characters", MaxLength)
	case IsReserved(s):
		return fmt.Errorf("%q is reserved for application routes", s)
	}

	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_':
		default:
			return fmt.Errorf("may only contain letters, digits, hyphens, and underscores")
		}
	}
	return nil
}

// Normalize trims a slug taken from a URL path.
func Normalize(s string) string {
	return strings.Trim(strings.TrimSpace(s), "/")
}
