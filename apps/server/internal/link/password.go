package link

import "github.com/rioprastiawan/shorturl/apps/server/internal/security"

// securityVerify is the real password check. It lives behind the
// verifyPassword variable in handler.go so tests can substitute a fast stub
// instead of paying Argon2's deliberate cost on every case.
func securityVerify(password, hash string) (bool, error) {
	return security.VerifyPassword(password, hash)
}
