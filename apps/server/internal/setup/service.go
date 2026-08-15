// Package setup implements the first-run wizard: the one moment where an
// unauthenticated caller may create an admin account.
//
// Everything it writes happens in a single transaction, so a partially
// initialised install — an admin with no workspace, or a workspace nobody owns
// — cannot exist, and the completion flag is only visible once the rest is.
package setup

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rioprastiawan/shorturl/apps/server/internal/auth"
	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
	"github.com/rioprastiawan/shorturl/apps/server/internal/database"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/security"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
	"github.com/rioprastiawan/shorturl/apps/server/internal/workspace"
)

// SettingKey is the app_settings row that records a finished wizard.
const SettingKey = "setup_completed"
const DeploymentModeKey = "deployment_mode"

const (
	ModeInternal = "internal"
	ModePublic   = "public"
)

// settingTrue is the JSONB payload written to that row.
var settingTrue = []byte("true")

// Service holds the first-run business rules.
type Service struct {
	pool *database.Pool
	q    *store.Queries
	cfg  config.Config
}

// NewService builds the setup service.
func NewService(pool *database.Pool, q *store.Queries, cfg config.Config) *Service {
	return &Service{pool: pool, q: q, cfg: cfg}
}

// Status reports whether the install has been initialised.
//
// The flag is authoritative, but the presence of any user also counts: an
// install that predates the flag, or one whose app_settings row was deleted,
// must not reopen a wizard that mints an admin account to anyone who asks.
func (s *Service) Status(ctx context.Context) (bool, error) {
	setting, err := s.q.GetSetting(ctx, SettingKey)
	switch {
	case err == nil:
		var completed bool
		if json.Unmarshal(setting.Value, &completed) == nil && completed {
			return true, nil
		}
	case errors.Is(err, pgx.ErrNoRows):
		// Not recorded; fall through to the user count.
	default:
		return false, httpx.Internal(err)
	}

	users, err := s.q.CountUsers(ctx)
	if err != nil {
		return false, httpx.Internal(err)
	}
	return users > 0, nil
}

// Complete creates the admin account, their first workspace, and a session,
// then marks setup as done. It returns the plaintext session token so the
// handler can log the new admin straight in.
//
// The caller is responsible for having validated the inputs; name and email
// must already be normalised.
func (s *Service) Complete(ctx context.Context, name, email, password, workspaceName, deploymentMode string) (*store.User, *store.Workspace, string, time.Time, error) {
	var noTime time.Time

	completed, err := s.Status(ctx)
	if err != nil {
		return nil, nil, "", noTime, err
	}
	if completed {
		return nil, nil, "", noTime, httpx.ErrSetupDone
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return nil, nil, "", noTime, httpx.Internal(err)
	}

	token, err := security.NewToken(auth.SessionTokenBytes)
	if err != nil {
		return nil, nil, "", noTime, httpx.Internal(err)
	}
	expiresAt := time.Now().Add(s.cfg.SessionTTL)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, "", noTime, httpx.Internal(err)
	}
	defer tx.Rollback(ctx)

	qtx := store.New(tx)

	// Re-check inside the transaction: two concurrent wizard submissions would
	// both pass the check above, and only one may create an admin.
	users, err := qtx.CountUsers(ctx)
	if err != nil {
		return nil, nil, "", noTime, httpx.Internal(err)
	}
	if users > 0 {
		return nil, nil, "", noTime, httpx.ErrSetupDone
	}

	user, err := qtx.CreateUser(ctx, store.CreateUserParams{
		Name:         name,
		Email:        email,
		PasswordHash: hash,
		IsAdmin:      true,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, nil, "", noTime, httpx.Conflictf("email_taken", "An account with that email already exists")
		}
		return nil, nil, "", noTime, httpx.Internal(err)
	}

	slug, err := workspace.UniqueSlug(ctx, qtx, workspaceName)
	if err != nil {
		return nil, nil, "", noTime, err
	}

	ws, err := qtx.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		Name:        workspaceName,
		Slug:        slug,
		OwnerUserID: user.ID,
	})
	if err != nil {
		return nil, nil, "", noTime, httpx.Internal(err)
	}

	if _, err := qtx.AddWorkspaceMember(ctx, store.AddWorkspaceMemberParams{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        string(authctx.RoleOwner),
	}); err != nil {
		return nil, nil, "", noTime, httpx.Internal(err)
	}

	if _, err := qtx.SetSetting(ctx, store.SetSettingParams{
		Key:   SettingKey,
		Value: settingTrue,
	}); err != nil {
		return nil, nil, "", noTime, httpx.Internal(err)
	}

	modeJSON, _ := json.Marshal(deploymentMode)
	if _, err := qtx.SetSetting(ctx, store.SetSettingParams{Key: DeploymentModeKey, Value: modeJSON}); err != nil {
		return nil, nil, "", noTime, httpx.Internal(err)
	}

	if _, err := qtx.CreateSession(ctx, store.CreateSessionParams{
		UserID:    user.ID,
		TokenHash: security.HashToken(token),
		ExpiresAt: expiresAt,
	}); err != nil {
		return nil, nil, "", noTime, httpx.Internal(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, "", noTime, httpx.Internal(err)
	}

	return &user, &ws, token, expiresAt, nil
}

// DeploymentMode returns the selected setup mode. Existing installations
// default to public so an upgrade does not unexpectedly disable registration.
func (s *Service) DeploymentMode(ctx context.Context) (string, error) {
	setting, err := s.q.GetSetting(ctx, DeploymentModeKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return ModePublic, nil
	}
	if err != nil {
		return "", httpx.Internal(err)
	}
	var mode string
	if json.Unmarshal(setting.Value, &mode) != nil || (mode != ModeInternal && mode != ModePublic) {
		return ModePublic, nil
	}
	return mode, nil
}
