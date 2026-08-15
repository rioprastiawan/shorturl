package authctx

import "testing"

// TestRolePermissionMatrix pins the authorization rules. If one of these ever
// flips, it is a privilege change and must be deliberate.
func TestRolePermissionMatrix(t *testing.T) {
	type perms struct {
		links, domains, members, apiKeys, workspace, actOnOwner bool
	}
	want := map[Role]perms{
		RoleOwner:  {links: true, domains: true, members: true, apiKeys: true, workspace: true, actOnOwner: true},
		RoleAdmin:  {links: true, domains: true, members: true, apiKeys: true, workspace: false, actOnOwner: false},
		RoleMember: {links: true, domains: false, members: false, apiKeys: false, workspace: false, actOnOwner: false},
	}

	for role, w := range want {
		t.Run(string(role), func(t *testing.T) {
			check := func(name string, got, expected bool) {
				t.Helper()
				if got != expected {
					t.Errorf("%s = %v, want %v", name, got, expected)
				}
			}
			check("CanManageLinks", role.CanManageLinks(), w.links)
			check("CanManageDomains", role.CanManageDomains(), w.domains)
			check("CanManageMembers", role.CanManageMembers(), w.members)
			check("CanManageAPIKeys", role.CanManageAPIKeys(), w.apiKeys)
			check("CanManageWorkspace", role.CanManageWorkspace(), w.workspace)
			check("CanActOnOwner", role.CanActOnOwner(), w.actOnOwner)
		})
	}
}

func TestUnknownRoleHasNoPrivileges(t *testing.T) {
	unknown := Role("superuser")
	if unknown.Valid() {
		t.Error("Valid() = true for an unrecognised role")
	}
	for name, got := range map[string]bool{
		"CanManageLinks":     unknown.CanManageLinks(),
		"CanManageDomains":   unknown.CanManageDomains(),
		"CanManageMembers":   unknown.CanManageMembers(),
		"CanManageAPIKeys":   unknown.CanManageAPIKeys(),
		"CanManageWorkspace": unknown.CanManageWorkspace(),
		"CanActOnOwner":      unknown.CanActOnOwner(),
	} {
		if got {
			t.Errorf("%s = true for an unrecognised role, want false", name)
		}
	}
}

func TestRoleAtLeast(t *testing.T) {
	if !RoleOwner.AtLeast(RoleAdmin) {
		t.Error("owner should outrank admin")
	}
	if !RoleAdmin.AtLeast(RoleMember) {
		t.Error("admin should outrank member")
	}
	if RoleMember.AtLeast(RoleAdmin) {
		t.Error("member must not outrank admin")
	}
	if !RoleMember.AtLeast(RoleMember) {
		t.Error("AtLeast should be inclusive of the same role")
	}
}

func TestAPIKeyScopes(t *testing.T) {
	p := APIKeyPrincipal{Scopes: []string{ScopeLinksRead, ScopeLinksWrite}}

	if !p.HasScope(ScopeLinksRead) {
		t.Error("HasScope(links:read) = false, want true")
	}
	if p.HasScope(ScopeAnalyticsRead) {
		t.Error("HasScope(analytics:read) = true for a key without it")
	}

	empty := APIKeyPrincipal{}
	if empty.HasScope(ScopeLinksRead) {
		t.Error("a key with no scopes granted one")
	}
}
