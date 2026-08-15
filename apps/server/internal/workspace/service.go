// Package workspace implements workspaces and their membership: the tenancy
// boundary every other resource hangs off.
package workspace

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/database"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

// errUnknownUser is a 404 that names the missing thing, because the caller
// typed the email and needs to know it did not match anyone. Membership is
// still not disclosed: the caller already has manage-members rights here.
var errUnknownUser = &httpx.APIError{
	Status:  http.StatusNotFound,
	Code:    "not_found",
	Message: "No user with that email address",
}

// Service holds the workspace business rules.
//
// It takes the pool as well as the queries because creating a workspace has to
// write two tables atomically; a workspace with no owner row would be
// invisible to the person who just created it.
type Service struct {
	pool *database.Pool
	q    *store.Queries
}

// NewService builds the workspace service.
func NewService(pool *database.Pool, q *store.Queries) *Service {
	return &Service{pool: pool, q: q}
}

// Create makes a workspace owned by the given user.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, name string) (*store.Workspace, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, httpx.Internal(err)
	}
	defer tx.Rollback(ctx)

	qtx := store.New(tx)

	slug, err := UniqueSlug(ctx, qtx, name)
	if err != nil {
		return nil, err
	}

	ws, err := qtx.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		Name:        name,
		Slug:        slug,
		OwnerUserID: userID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, httpx.Conflictf("slug_taken", "A workspace with that name already exists")
		}
		return nil, httpx.Internal(err)
	}

	if _, err := qtx.AddWorkspaceMember(ctx, store.AddWorkspaceMemberParams{
		WorkspaceID: ws.ID,
		UserID:      userID,
		Role:        string(authctx.RoleOwner),
	}); err != nil {
		return nil, httpx.Internal(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, httpx.Internal(err)
	}
	return &ws, nil
}

// ListForUser returns every workspace the user belongs to, with their role.
func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID) ([]WorkspaceWithRole, error) {
	rows, err := s.q.ListWorkspacesForUser(ctx, userID)
	if err != nil {
		return nil, httpx.Internal(err)
	}

	out := make([]WorkspaceWithRole, 0, len(rows))
	for _, row := range rows {
		out = append(out, WorkspaceWithRole{Workspace: row.Workspace, Role: authctx.Role(row.Role)})
	}
	return out, nil
}

// Get returns a single workspace. Access is decided by the membership the
// middleware resolved, so this only maps a missing row.
func (s *Service) Get(ctx context.Context, workspaceID uuid.UUID) (*store.Workspace, error) {
	ws, err := s.q.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, notFoundOrInternal(err)
	}
	return &ws, nil
}

// Update renames a workspace. The slug is deliberately left alone: it appears
// in URLs that may already be shared.
func (s *Service) Update(ctx context.Context, workspaceID uuid.UUID, name string) (*store.Workspace, error) {
	ws, err := s.q.UpdateWorkspace(ctx, store.UpdateWorkspaceParams{ID: workspaceID, Name: name})
	if err != nil {
		return nil, notFoundOrInternal(err)
	}
	return &ws, nil
}

// Delete removes a workspace and, by cascade, everything inside it.
//
// The owner's last workspace is refused: the dashboard has no state for a user
// with nowhere to go, and the deletion is irreversible.
func (s *Service) Delete(ctx context.Context, workspaceID, userID uuid.UUID) error {
	count, err := s.q.CountWorkspacesForUser(ctx, userID)
	if err != nil {
		return httpx.Internal(err)
	}
	if count <= 1 {
		return httpx.Conflictf("last_workspace", "You cannot delete your only workspace")
	}

	if err := s.q.DeleteWorkspace(ctx, workspaceID); err != nil {
		return httpx.Internal(err)
	}
	return nil
}

// ListMembers returns the workspace's members, owner first.
func (s *Service) ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]Member, error) {
	rows, err := s.q.ListWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return nil, httpx.Internal(err)
	}

	out := make([]Member, 0, len(rows))
	for _, row := range rows {
		out = append(out, Member{
			UserID:    row.UserID,
			Name:      row.Name,
			Email:     row.Email,
			Role:      authctx.Role(row.Role),
			CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

// MemberRole returns a member's current role, which the handler needs before
// it can decide whether the caller may act on them.
func (s *Service) MemberRole(ctx context.Context, workspaceID, userID uuid.UUID) (authctx.Role, error) {
	member, err := s.q.GetWorkspaceMember(ctx, store.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		UserID:      userID,
	})
	if err != nil {
		return "", notFoundOrInternal(err)
	}
	return authctx.Role(member.Role), nil
}

// AddMemberByEmail adds an existing account to the workspace.
//
// There are no invitations in the MVP, so the user must already have signed
// up; that also keeps this endpoint from becoming a way to send mail.
func (s *Service) AddMemberByEmail(ctx context.Context, workspaceID uuid.UUID, email string, role authctx.Role) (*Member, error) {
	user, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errUnknownUser
		}
		return nil, httpx.Internal(err)
	}

	member, err := s.q.AddWorkspaceMember(ctx, store.AddWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		UserID:      user.ID,
		Role:        string(role),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, httpx.Conflictf("member_exists", "That user is already a member of this workspace")
		}
		return nil, httpx.Internal(err)
	}

	return &Member{
		UserID:    user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Role:      authctx.Role(member.Role),
		CreatedAt: member.CreatedAt,
	}, nil
}

// UpdateMemberRole changes an existing member's role.
func (s *Service) UpdateMemberRole(ctx context.Context, workspaceID, userID uuid.UUID, role authctx.Role) (*Member, error) {
	member, err := s.q.UpdateWorkspaceMemberRole(ctx, store.UpdateWorkspaceMemberRoleParams{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        string(role),
	})
	if err != nil {
		return nil, notFoundOrInternal(err)
	}

	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return nil, notFoundOrInternal(err)
	}

	return &Member{
		UserID:    user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Role:      authctx.Role(member.Role),
		CreatedAt: member.CreatedAt,
	}, nil
}

// RemoveMember drops a membership. Links the member created stay, because they
// belong to the workspace rather than to them.
func (s *Service) RemoveMember(ctx context.Context, workspaceID, userID uuid.UUID) error {
	rows, err := s.q.RemoveWorkspaceMember(ctx, store.RemoveWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		UserID:      userID,
	})
	if err != nil {
		return httpx.Internal(err)
	}
	if rows == 0 {
		return httpx.ErrNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// notFoundOrInternal keeps raw pgx errors from reaching the client.
func notFoundOrInternal(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.ErrNotFound
	}
	return httpx.Internal(err)
}
