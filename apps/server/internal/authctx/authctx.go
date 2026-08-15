// Package authctx carries the authenticated principal through the request
// context and defines what each role may do.
//
// Every feature package reads authorization from here rather than deriving it
// independently, so there is exactly one definition of "can this caller do
// this" in the codebase.
package authctx

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

type ctxKey int

const (
	userKey ctxKey = iota
	membershipKey
	apiKeyKey
)

// Role is a workspace membership level.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// rank orders roles so comparisons stay readable.
func (r Role) rank() int {
	switch r {
	case RoleOwner:
		return 3
	case RoleAdmin:
		return 2
	case RoleMember:
		return 1
	default:
		return 0
	}
}

// Valid reports whether the role is one the system recognises.
func (r Role) Valid() bool { return r.rank() > 0 }

// AtLeast reports whether this role includes the privileges of other.
func (r Role) AtLeast(other Role) bool { return r.rank() >= other.rank() }

// CanManageLinks: every member can create and edit links. Read-only access is
// not a role in the MVP.
func (r Role) CanManageLinks() bool { return r.rank() >= RoleMember.rank() }

// CanManageDomains restricts DNS and TLS changes to admins and owners.
func (r Role) CanManageDomains() bool { return r.AtLeast(RoleAdmin) }

// CanManageMembers restricts the member list to admins and owners. Acting on
// an owner additionally requires CanActOnOwner.
func (r Role) CanManageMembers() bool { return r.AtLeast(RoleAdmin) }

// CanManageAPIKeys restricts key issuance to admins and owners, because a key
// grants workspace-wide access that outlives any session.
func (r Role) CanManageAPIKeys() bool { return r.AtLeast(RoleAdmin) }

// CanManageWorkspace covers renaming and deleting the workspace.
func (r Role) CanManageWorkspace() bool { return r == RoleOwner }

// CanActOnOwner reports whether this role may change or remove the owner.
// Only the owner can, which prevents an admin from demoting the person who
// owns the workspace.
func (r Role) CanActOnOwner() bool { return r == RoleOwner }

// Membership is the caller's relationship to the workspace being addressed.
type Membership struct {
	WorkspaceID uuid.UUID
	UserID      uuid.UUID
	Role        Role
}

// APIKeyPrincipal is a machine caller authenticated by bearer token.
type APIKeyPrincipal struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Scopes      []string
}

// HasScope reports whether the key carries the named scope.
func (p APIKeyPrincipal) HasScope(scope string) bool {
	return slices.Contains(p.Scopes, scope)
}

// Known API-key scopes.
const (
	ScopeLinksRead     = "links:read"
	ScopeLinksWrite    = "links:write"
	ScopeAnalyticsRead = "analytics:read"
)

// ValidScopes lists every scope the API accepts.
var ValidScopes = []string{ScopeLinksRead, ScopeLinksWrite, ScopeAnalyticsRead}

// WithUser attaches the authenticated dashboard user.
func WithUser(ctx context.Context, user *store.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// User returns the authenticated dashboard user, if any.
func User(ctx context.Context) (*store.User, bool) {
	u, ok := ctx.Value(userKey).(*store.User)
	return u, ok && u != nil
}

// MustUser returns the authenticated user. Only call this behind the
// authentication middleware, which guarantees one is present.
func MustUser(ctx context.Context) *store.User {
	u, ok := User(ctx)
	if !ok {
		panic("authctx: no user in context; handler is missing RequireAuth middleware")
	}
	return u
}

// WithMembership attaches the resolved workspace membership.
func WithMembership(ctx context.Context, m Membership) context.Context {
	return context.WithValue(ctx, membershipKey, m)
}

// CurrentMembership returns the resolved workspace membership, if any.
func CurrentMembership(ctx context.Context) (Membership, bool) {
	m, ok := ctx.Value(membershipKey).(Membership)
	return m, ok
}

// MustMembership returns the workspace membership. Only call this behind the
// workspace authorization middleware.
func MustMembership(ctx context.Context) Membership {
	m, ok := CurrentMembership(ctx)
	if !ok {
		panic("authctx: no membership in context; handler is missing RequireWorkspace middleware")
	}
	return m
}

// WithAPIKey attaches the authenticated machine caller.
func WithAPIKey(ctx context.Context, key APIKeyPrincipal) context.Context {
	return context.WithValue(ctx, apiKeyKey, key)
}

// APIKey returns the authenticated machine caller, if any.
func APIKey(ctx context.Context) (APIKeyPrincipal, bool) {
	k, ok := ctx.Value(apiKeyKey).(APIKeyPrincipal)
	return k, ok
}

// MustAPIKey returns the machine caller. Only call this behind the API-key
// authentication middleware.
func MustAPIKey(ctx context.Context) APIKeyPrincipal {
	k, ok := APIKey(ctx)
	if !ok {
		panic("authctx: no API key in context; handler is missing RequireAPIKey middleware")
	}
	return k
}
