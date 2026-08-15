package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
)

// NewToken returns a URL-safe random token with n bytes of entropy.
func NewToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken returns the SHA-256 hex digest of an opaque token.
//
// Session cookies and API keys are high-entropy random values, not passwords,
// so a fast digest is the right tool: there is nothing to brute-force, and the
// hash must be cheap enough to compute on every request. Storing only the
// digest means a database leak does not yield usable credentials.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// HashIP anonymises a client address with a keyed HMAC. The secret never
// leaves the server, so the output cannot be reversed by dictionary attack
// over the small IPv4 space the way a plain hash could.
//
// Returns an empty string for an unparseable address rather than hashing
// garbage, so bad input does not pollute analytics with distinct values.
func HashIP(secret []byte, addr string) string {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return ""
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(ip.String()))
	return hex.EncodeToString(mac.Sum(nil))
}
