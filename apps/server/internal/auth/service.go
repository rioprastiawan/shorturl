// Package auth implements cookie-session authentication for the dashboard:
// registration, login, logout, and resolving a cookie back to a user.
//
// Sessions are server-side. The cookie carries an opaque random token and the
// database stores only its SHA-256 digest, so a dump of the sessions table
// yields nothing a caller could replay.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
	"github.com/rioprastiawan/shorturl/apps/server/internal/database"
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

var errTwoFactorRequired = &httpx.APIError{Status: http.StatusForbidden, Code: "two_factor_required", Message: "Enter your authentication code"}
var errInvalidTwoFactor = httpx.Invalid(map[string][]string{"code": {"is invalid or has expired"}})

// dummyPasswordHash is verified against when the email is unknown, so that a
// login attempt costs the same Argon2 work whether or not the account exists
// and the response time does not leak which emails are registered. The value
// is a real hash of a random string that is not a usable password.
const dummyPasswordHash = "$argon2id$v=19$m=65536,t=2,p=4$QocpZjr674eGqTwTa97TxQ$6oXXfcTxPSuYAJ7NkCivrvoeoXRCbk9fYO0ue6pzbQk"

// Service holds the authentication business rules.
type Service struct {
	pool *database.Pool
	q    *store.Queries
	cfg  config.Config
}

// NewService builds the authentication service.
func NewService(pool *database.Pool, q *store.Queries, cfg config.Config) *Service {
	return &Service{pool: pool, q: q, cfg: cfg}
}

// Register creates a user with a hashed password. The caller is responsible
// for having validated the inputs.
func (s *Service) Register(ctx context.Context, name, email, password, invitationToken string) (*store.User, error) {
	hash, err := security.HashPassword(password)
	if err != nil {
		return nil, httpx.Internal(err)
	}

	if invitationToken == "" {
		enabled, err := s.publicRegistrationEnabled(ctx)
		if err != nil {
			return nil, err
		}
		if !enabled {
			return nil, &httpx.APIError{Status: http.StatusForbidden, Code: "registration_disabled", Message: "Public registration is disabled on this installation"}
		}
		return s.createUser(ctx, s.q, name, email, hash)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, httpx.Internal(err)
	}
	defer tx.Rollback(ctx)

	var workspaceID uuid.UUID
	var role string
	err = tx.QueryRow(ctx, `SELECT workspace_id, role FROM workspace_invitations
		WHERE token_hash = $1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > now()
		FOR UPDATE`, security.HashToken(invitationToken)).Scan(&workspaceID, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.Invalid(map[string][]string{"invitation_token": {"is invalid or has expired"}})
	}
	if err != nil {
		return nil, httpx.Internal(err)
	}

	qtx := store.New(tx)
	user, err := s.createUser(ctx, qtx, name, email, hash)
	if err != nil {
		return nil, err
	}
	if _, err := qtx.AddWorkspaceMember(ctx, store.AddWorkspaceMemberParams{
		WorkspaceID: workspaceID, UserID: user.ID, Role: role,
	}); err != nil {
		return nil, httpx.Internal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE workspace_invitations SET accepted_at = now()
		WHERE token_hash = $1`, security.HashToken(invitationToken)); err != nil {
		return nil, httpx.Internal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, httpx.Internal(err)
	}
	return user, nil
}

type userCreator interface {
	CreateUser(context.Context, store.CreateUserParams) (store.User, error)
}

func (s *Service) createUser(ctx context.Context, q userCreator, name, email, hash string) (*store.User, error) {
	user, err := q.CreateUser(ctx, store.CreateUserParams{
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

func (s *Service) publicRegistrationEnabled(ctx context.Context) (bool, error) {
	setting, err := s.q.GetSetting(ctx, "deployment_mode")
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, httpx.Internal(err)
	}
	var mode string
	if json.Unmarshal(setting.Value, &mode) != nil {
		return true, nil
	}
	return mode != "internal", nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, name, email string) (*store.User, error) {
	user, err := s.q.UpdateUserProfile(ctx, store.UpdateUserProfileParams{ID: userID, Name: name, Email: email})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, httpx.Conflictf("email_taken", "An account with that email already exists")
		}
		return nil, httpx.Internal(err)
	}
	return &user, nil
}

func (s *Service) UpdatePreferences(ctx context.Context, userID uuid.UUID, language, timezone string) (*store.User, error) {
	user, err := s.q.UpdateUserPreferences(ctx, store.UpdateUserPreferencesParams{
		ID: userID, Language: language, Timezone: timezone,
	})
	if err != nil {
		return nil, httpx.Internal(err)
	}
	return &user, nil
}

func (s *Service) ChangePassword(ctx context.Context, user *store.User, currentPassword, newPassword, currentToken string) error {
	ok, err := security.VerifyPassword(currentPassword, user.PasswordHash)
	if err != nil {
		return httpx.Internal(err)
	}
	if !ok {
		return httpx.Invalid(map[string][]string{"current_password": {"is incorrect"}})
	}
	if currentPassword == newPassword {
		return httpx.Invalid(map[string][]string{"new_password": {"must be different from your current password"}})
	}
	hash, err := security.HashPassword(newPassword)
	if err != nil {
		return httpx.Internal(err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return httpx.Internal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, user.ID, hash); err != nil {
		return httpx.Internal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND token_hash <> $2`, user.ID, security.HashToken(currentToken)); err != nil {
		return httpx.Internal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return httpx.Internal(err)
	}
	return nil
}

// Login verifies credentials and opens a session, returning the plaintext
// token to put in the cookie. Only the token's digest is persisted.
func (s *Service) Login(ctx context.Context, email, password, code, userAgent, ipHash string) (*store.User, string, time.Time, error) {
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
	enabled, err := s.TwoFactorStatus(ctx, user.ID)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	if enabled {
		if strings.TrimSpace(code) == "" {
			return nil, "", time.Time{}, errTwoFactorRequired
		}
		if err := s.verifySecondFactor(ctx, user.ID, code, true, true); err != nil {
			return nil, "", time.Time{}, err
		}
	}

	token, expiresAt, err := s.StartSession(ctx, user.ID, userAgent, ipHash)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	return &user, token, expiresAt, nil
}

func (s *Service) TwoFactorStatus(ctx context.Context, userID uuid.UUID) (bool, error) {
	var enabled bool
	err := s.pool.QueryRow(ctx, `SELECT enabled FROM user_two_factor WHERE user_id = $1`, userID).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, httpx.Internal(err)
	}
	return enabled, nil
}

func (s *Service) BeginTwoFactor(ctx context.Context, user *store.User, password string) (twoFactorSetupDTO, error) {
	enabled, err := s.TwoFactorStatus(ctx, user.ID)
	if err != nil {
		return twoFactorSetupDTO{}, err
	}
	if enabled {
		return twoFactorSetupDTO{}, httpx.Conflictf("two_factor_enabled", "Two-step verification is already enabled")
	}
	ok, err := security.VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return twoFactorSetupDTO{}, httpx.Internal(err)
	}
	if !ok {
		return twoFactorSetupDTO{}, httpx.Invalid(map[string][]string{"password": {"is incorrect"}})
	}
	secret, err := security.NewTOTPSecret()
	if err != nil {
		return twoFactorSetupDTO{}, httpx.Internal(err)
	}
	ciphertext, err := security.EncryptSecret(s.cfg.SessionSecret, secret)
	if err != nil {
		return twoFactorSetupDTO{}, httpx.Internal(err)
	}
	codes := make([]string, 8)
	hashes := make([]string, 8)
	for i := range codes {
		codes[i], err = security.RecoveryCode()
		if err != nil {
			return twoFactorSetupDTO{}, httpx.Internal(err)
		}
		hashes[i] = security.HashRecoveryCode(codes[i])
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO user_two_factor (user_id, secret_ciphertext, recovery_code_hashes, enabled)
		VALUES ($1,$2,$3,false) ON CONFLICT (user_id) DO UPDATE SET secret_ciphertext=$2,recovery_code_hashes=$3,enabled=false`, user.ID, ciphertext, hashes)
	if err != nil {
		return twoFactorSetupDTO{}, httpx.Internal(err)
	}
	issuer := url.QueryEscape(s.cfg.AppName)
	labelIssuer := url.PathEscape(s.cfg.AppName)
	account := url.PathEscape(user.Email)
	uri := fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30", labelIssuer, account, secret, issuer)
	return twoFactorSetupDTO{Secret: secret, URI: uri, RecoveryCodes: codes}, nil
}

func (s *Service) EnableTwoFactor(ctx context.Context, userID uuid.UUID, code string) error {
	if err := s.verifySecondFactor(ctx, userID, code, false, false); err != nil {
		return err
	}
	result, err := s.pool.Exec(ctx, `UPDATE user_two_factor SET enabled=true WHERE user_id=$1`, userID)
	if err != nil {
		return httpx.Internal(err)
	}
	if result.RowsAffected() == 0 {
		return httpx.Conflictf("two_factor_not_configured", "Start setup first")
	}
	return nil
}

func (s *Service) DisableTwoFactor(ctx context.Context, user *store.User, password, code string) error {
	ok, err := security.VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return httpx.Internal(err)
	}
	if !ok {
		return httpx.Invalid(map[string][]string{"password": {"is incorrect"}})
	}
	if err := s.verifySecondFactor(ctx, user.ID, code, true, true); err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM user_two_factor WHERE user_id=$1`, user.ID)
	if err != nil {
		return httpx.Internal(err)
	}
	return nil
}

func (s *Service) verifySecondFactor(ctx context.Context, userID uuid.UUID, code string, allowRecovery, consumeRecovery bool) error {
	var encrypted []byte
	var hashes []string
	err := s.pool.QueryRow(ctx, `SELECT secret_ciphertext,recovery_code_hashes FROM user_two_factor WHERE user_id=$1`, userID).Scan(&encrypted, &hashes)
	if errors.Is(err, pgx.ErrNoRows) {
		return errInvalidTwoFactor
	}
	if err != nil {
		return httpx.Internal(err)
	}
	secret, err := security.DecryptSecret(s.cfg.SessionSecret, encrypted)
	if err != nil {
		return httpx.Internal(err)
	}
	if security.ValidateTOTP(secret, code, time.Now()) {
		return nil
	}
	if !allowRecovery {
		return errInvalidTwoFactor
	}
	hash := security.HashRecoveryCode(code)
	for _, candidate := range hashes {
		if candidate == hash {
			if consumeRecovery {
				var consumed bool
				err = s.pool.QueryRow(ctx, `UPDATE user_two_factor SET recovery_code_hashes=array_remove(recovery_code_hashes,$2)
					WHERE user_id=$1 AND $2=ANY(recovery_code_hashes) RETURNING true`, userID, hash).Scan(&consumed)
				if errors.Is(err, pgx.ErrNoRows) {
					return errInvalidTwoFactor
				}
				if err != nil {
					return httpx.Internal(err)
				}
			}
			return nil
		}
	}
	return errInvalidTwoFactor
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
