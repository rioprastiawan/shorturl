// Package auth implements cookie-session authentication for the dashboard:
// registration, login, logout, and resolving a cookie back to a user.
//
// Sessions are server-side. The cookie carries an opaque random token and the
// database stores only its SHA-256 digest, so a dump of the sessions table
// yields nothing a caller could replay.
package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/security"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

// SessionTokenBytes is the entropy of a session token. 32 bytes is well beyond
// what an online guessing attack could cover.
const SessionTokenBytes = 32

// maxUserAgentLength bounds what is copied into the sessions row, since the
// header is attacker-controlled and only used to label a device.
const maxUserAgentLength = 512

// errInvalidCredentials is returned for both an unknown email and a wrong
// password. A distinct "no such account" response would turn the login
// endpoint into an account-existence oracle.
var errInvalidCredentials = &httpx.APIError{
	Status:  http.StatusUnauthorized,
	Code:    "unauthorized",
	Message: "Invalid email or password",
}

// dummyPasswordHash is verified against when the email is unknown, so that a
// login attempt costs the same Argon2 work whether or not the account exists
// and the response time does not leak which emails are registered. The value
// is a real hash of a random string that is not a usable password.
const dummyPasswordHash = "$argon2id$v=19$m=65536,t=2,p=4$QocpZjr674eGqTwTa97TxQ$6oXXfcTxPSuYAJ7NkCivrvoeoXRCbk9fYO0ue6pzbQk"

// Service holds the authentication business rules.
type Service struct {
	q   *store.Queries
	cfg config.Config
}

// NewService builds the authentication service.
func NewService(q *store.Queries, cfg config.Config) *Service {
	return &Service{q: q, cfg: cfg}
}

// Register creates a user with a hashed password. The caller is responsible
// for having validated the inputs.
func (s *Service) Register(ctx context.Context, name, email, password string) (*store.User, error) {
	hash, err := security.HashPassword(password)
	if err != nil {
		return nil, httpx.Internal(err)
	}

	user, err := s.q.CreateUser(ctx, store.CreateUserParams{
		Name:         name,
		Email:        email,
		PasswordHash: hash,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, httpx.Conflictf("email_taken", "An account with that email already exists")
		}
		return nil, httpx.Internal(err)
	}
	return &user, nil
}

// Login verifies credentials and opens a session, returning the plaintext
// token to put in the cookie. Only the token's digest is persisted.
func (s *Service) Login(ctx context.Context, email, password, userAgent, ipHash string) (*store.User, string, time.Time, error) {
	user, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Spend the same time as a real verification before failing.
			_, _ = security.VerifyPassword(password, dummyPasswordHash)
			return nil, "", time.Time{}, errInvalidCredentials
		}
		return nil, "", time.Time{}, httpx.Internal(err)
	}

	ok, err := security.VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return nil, "", time.Time{}, httpx.Internal(err)
	}
	if !ok {
		return nil, "", time.Time{}, errInvalidCredentials
	}

	token, expiresAt, err := s.StartSession(ctx, user.ID, userAgent, ipHash)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	return &user, token, expiresAt, nil
}

// StartSession issues a session for an already-authenticated user. Register
// uses it to log the new account in without a second password check.
func (s *Service) StartSession(ctx context.Context, userID uuid.UUID, userAgent, ipHash string) (string, time.Time, error) {
	token, err := security.NewToken(SessionTokenBytes)
	if err != nil {
		return "", time.Time{}, httpx.Internal(err)
	}

	expiresAt := time.Now().Add(s.cfg.SessionTTL)
	if _, err := s.q.CreateSession(ctx, store.CreateSessionParams{
		UserID:    userID,
		TokenHash: security.HashToken(token),
		UserAgent: optional(truncate(userAgent, maxUserAgentLength)),
		IpHash:    optional(ipHash),
		ExpiresAt: expiresAt,
	}); err != nil {
		return "", time.Time{}, httpx.Internal(err)
	}
	return token, expiresAt, nil
}

// Logout deletes the session behind a plaintext token. An unknown token is not
// an error: the caller ends up logged out either way.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := s.q.DeleteSession(ctx, security.HashToken(token)); err != nil {
		return httpx.Internal(err)
	}
	return nil
}

// Authenticate resolves a plaintext session token to its user. Expired
// sessions are filtered by the query, so anything that does not resolve is
// simply unauthorized.
func (s *Service) Authenticate(ctx context.Context, token string) (*store.User, error) {
	if token == "" {
		return nil, httpx.ErrUnauthorized
	}

	row, err := s.q.GetSessionWithUser(ctx, security.HashToken(token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.ErrUnauthorized
		}
		return nil, httpx.Internal(err)
	}

	// Best effort: the request is authenticated whether or not last_seen_at
	// could be refreshed, so a failure here is logged rather than surfaced.
	if err := s.q.TouchSession(ctx, row.Session.ID); err != nil {
		slog.WarnContext(ctx, "touch session", slog.String("error", err.Error()))
	}

	user := row.User
	return &user, nil
}

// optional maps an empty string to a NULL column.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}
