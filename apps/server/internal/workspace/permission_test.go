package workspace

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
)

var (
	ownerID  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	adminID  = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	memberID = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	otherID  = uuid.MustParse("44444444-4444-4444-4444-444444444444")
)

const workspaceID = "55555555-5555-5555-5555-555555555555"

func actor(userID uuid.UUID, role authctx.Role) authctx.Membership {
	return authctx.Membership{
		WorkspaceID: uuid.MustParse(workspaceID),
		UserID:      userID,
		Role:        role,
	}
}

// result describes the outcome a caller should see: "" means allowed.
func result(err error) (code string, status int) {
	if err == nil {
		return "", 0
	}
	apiErr := httpx.AsAPIError(err)
	return apiErr.Code, apiErr.Status
}

func TestAuthorizeMemberChange(t *testing.T) {
	tests := []struct {
		name       string
		actor      authctx.Membership
		targetID   uuid.UUID
		targetRole authctx.Role
		action     memberAction
		wantCode   string
		wantStatus int
	}{
		{
			name:  "owner promotes a member",
			actor: actor(ownerID, authctx.RoleOwner), targetID: memberID, targetRole: authctx.RoleMember,
			action: actionUpdateRole,
		},
		{
			name:  "admin promotes a member",
			actor: actor(adminID, authctx.RoleAdmin), targetID: memberID, targetRole: authctx.RoleMember,
			action: actionUpdateRole,
		},
		{
			name:  "admin demotes another admin",
			actor: actor(adminID, authctx.RoleAdmin), targetID: otherID, targetRole: authctx.RoleAdmin,
			action: actionUpdateRole,
		},
		{
			name:  "member cannot change anyone's role",
			actor: actor(memberID, authctx.RoleMember), targetID: otherID, targetRole: authctx.RoleMember,
			action: actionUpdateRole, wantCode: "forbidden", wantStatus: http.StatusForbidden,
		},
		{
			name:  "member cannot change their own role",
			actor: actor(memberID, authctx.RoleMember), targetID: memberID, targetRole: authctx.RoleMember,
			action: actionUpdateRole, wantCode: "forbidden", wantStatus: http.StatusForbidden,
		},
		{
			name:  "admin cannot demote the owner and is not told why",
			actor: actor(adminID, authctx.RoleAdmin), targetID: ownerID, targetRole: authctx.RoleOwner,
			action: actionUpdateRole, wantCode: "forbidden", wantStatus: http.StatusForbidden,
		},
		{
			name:  "owner cannot demote themselves",
			actor: actor(ownerID, authctx.RoleOwner), targetID: ownerID, targetRole: authctx.RoleOwner,
			action: actionUpdateRole, wantCode: "owner_immutable", wantStatus: http.StatusConflict,
		},
		{
			name:  "owner removes a member",
			actor: actor(ownerID, authctx.RoleOwner), targetID: memberID, targetRole: authctx.RoleMember,
			action: actionRemove,
		},
		{
			name:  "admin removes a member",
			actor: actor(adminID, authctx.RoleAdmin), targetID: memberID, targetRole: authctx.RoleMember,
			action: actionRemove,
		},
		{
			name:  "member removes themselves",
			actor: actor(memberID, authctx.RoleMember), targetID: memberID, targetRole: authctx.RoleMember,
			action: actionRemove,
		},
		{
			name:  "admin removes themselves",
			actor: actor(adminID, authctx.RoleAdmin), targetID: adminID, targetRole: authctx.RoleAdmin,
			action: actionRemove,
		},
		{
			name:  "member cannot remove someone else",
			actor: actor(memberID, authctx.RoleMember), targetID: otherID, targetRole: authctx.RoleMember,
			action: actionRemove, wantCode: "forbidden", wantStatus: http.StatusForbidden,
		},
		{
			name:  "member cannot remove the owner",
			actor: actor(memberID, authctx.RoleMember), targetID: ownerID, targetRole: authctx.RoleOwner,
			action: actionRemove, wantCode: "forbidden", wantStatus: http.StatusForbidden,
		},
		{
			name:  "admin cannot remove the owner",
			actor: actor(adminID, authctx.RoleAdmin), targetID: ownerID, targetRole: authctx.RoleOwner,
			action: actionRemove, wantCode: "forbidden", wantStatus: http.StatusForbidden,
		},
		{
			name:  "owner cannot remove themselves",
			actor: actor(ownerID, authctx.RoleOwner), targetID: ownerID, targetRole: authctx.RoleOwner,
			action: actionRemove, wantCode: "owner_immutable", wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, status := result(authorizeMemberChange(tt.actor, tt.targetID, tt.targetRole, tt.action))
			if code != tt.wantCode || status != tt.wantStatus {
				t.Errorf("authorizeMemberChange(...) = (%q, %d), want (%q, %d)",
					code, status, tt.wantCode, tt.wantStatus)
			}
		})
	}
}

func TestAuthorizeWorkspaceChange(t *testing.T) {
	tests := []struct {
		name     string
		role     authctx.Role
		wantCode string
	}{
		{"owner may rename and delete", authctx.RoleOwner, ""},
		{"admin may not", authctx.RoleAdmin, "forbidden"},
		{"member may not", authctx.RoleMember, "forbidden"},
		{"unknown role may not", authctx.Role("viewer"), "forbidden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _ := result(authorizeWorkspaceChange(actor(ownerID, tt.role)))
			if code != tt.wantCode {
				t.Errorf("authorizeWorkspaceChange(%q) = %q, want %q", tt.role, code, tt.wantCode)
			}
		})
	}
}

func TestParseRole(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    authctx.Role
		wantErr bool
	}{
		{"admin", "admin", authctx.RoleAdmin, false},
		{"member", "member", authctx.RoleMember, false},
		// Owner is set when the workspace is created and never assigned here,
		// or a workspace could end up with two.
		{"owner is not assignable", "owner", "", true},
		{"empty", "", "", true},
		{"unknown", "superuser", "", true},
		{"wrong case", "Admin", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRole(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseRole(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseRole(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if tt.wantErr {
				if _, status := result(err); status != http.StatusUnprocessableEntity {
					t.Errorf("parseRole(%q) status = %d, want %d", tt.in, status, http.StatusUnprocessableEntity)
				}
			}
		})
	}
}
