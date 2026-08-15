package workspace

import (
	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
)

// errOwnerImmutable is returned for any attempt to demote or remove the owner.
//
// The owner is also workspaces.owner_user_id, so changing the membership row
// alone would leave the two out of step. Ownership transfer is a deliberate
// operation the MVP does not have, and silently allowing half of it would let
// a workspace end up with nobody who can manage it.
var errOwnerImmutable = httpx.Conflictf("owner_immutable", "Transfer ownership before changing the owner's role")

// memberAction distinguishes the two ways a membership can be touched, because
// removing yourself is allowed where changing your own role is not.
type memberAction int

const (
	actionUpdateRole memberAction = iota
	actionRemove
)

// assignableRoles are the roles a member may be given. Owner is set once, when
// the workspace is created, and never through the members API.
var assignableRoles = []authctx.Role{authctx.RoleAdmin, authctx.RoleMember}

// parseRole validates a role from a request body.
func parseRole(raw string) (authctx.Role, error) {
	role := authctx.Role(raw)
	for _, allowed := range assignableRoles {
		if role == allowed {
			return role, nil
		}
	}
	return "", httpx.Invalid(map[string][]string{
		"role": {"must be one of: admin, member"},
	})
}

// authorizeMemberChange decides whether actor may perform action on the member
// identified by targetID, whose current role is targetRole. It returns nil when
// the change is allowed and the error to render otherwise.
//
// Kept separate from the handler so the whole matrix is testable without HTTP.
func authorizeMemberChange(actor authctx.Membership, targetID uuid.UUID, targetRole authctx.Role, action memberAction) error {
	leavingSelf := action == actionRemove && actor.UserID == targetID

	// Anyone may leave a workspace they joined; everything else is a
	// member-management operation.
	if !leavingSelf && !actor.Role.CanManageMembers() {
		return httpx.ErrForbidden
	}

	if targetRole == authctx.RoleOwner {
		// An admin must not learn that the target is the owner by getting a
		// different error than for any other refusal.
		if !actor.Role.CanActOnOwner() {
			return httpx.ErrForbidden
		}
		return errOwnerImmutable
	}

	return nil
}

// authorizeWorkspaceChange gates renaming and deleting the workspace.
func authorizeWorkspaceChange(actor authctx.Membership) error {
	if !actor.Role.CanManageWorkspace() {
		return httpx.ErrForbidden
	}
	return nil
}
