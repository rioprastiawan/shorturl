package workspace

import (
	"time"

	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

// Name length bounds, matching workspaces.name VARCHAR(120).
const (
	NameMinLength = 2
	NameMaxLength = 120
)

// WorkspaceWithRole pairs a workspace with the caller's role in it, which is
// what every workspace response needs and neither table carries alone.
type WorkspaceWithRole struct {
	Workspace store.Workspace
	Role      authctx.Role
}

// Member is a workspace membership joined with the user it refers to.
type Member struct {
	UserID    uuid.UUID
	Name      string
	Email     string
	Role      authctx.Role
	CreatedAt time.Time
}

// DTO is the public shape of a workspace.
type DTO struct {
	ID        uuid.UUID    `json:"id"`
	Name      string       `json:"name"`
	Slug      string       `json:"slug"`
	Role      authctx.Role `json:"role"`
	CreatedAt time.Time    `json:"created_at"`
}

// NewDTO projects a workspace and the caller's role onto the public shape.
func NewDTO(ws *store.Workspace, role authctx.Role) DTO {
	return DTO{
		ID:        ws.ID,
		Name:      ws.Name,
		Slug:      ws.Slug,
		Role:      role,
		CreatedAt: ws.CreatedAt,
	}
}

// MemberDTO is the public shape of a workspace member.
type MemberDTO struct {
	UserID    uuid.UUID    `json:"user_id"`
	Name      string       `json:"name"`
	Email     string       `json:"email"`
	Role      authctx.Role `json:"role"`
	CreatedAt time.Time    `json:"created_at"`
}

// NewMemberDTO projects a member onto the public shape.
func NewMemberDTO(m Member) MemberDTO {
	return MemberDTO{
		UserID:    m.UserID,
		Name:      m.Name,
		Email:     m.Email,
		Role:      m.Role,
		CreatedAt: m.CreatedAt,
	}
}

type createRequest struct {
	Name string `json:"name"`
}

type updateRequest struct {
	Name string `json:"name"`
}

type addMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type updateMemberRequest struct {
	Role string `json:"role"`
}
