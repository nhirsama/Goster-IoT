package identity

import (
	"context"
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

type stubTenantRoleStore struct {
	roles map[string]map[string]inter.TenantRole
	err   error
}

func (s stubTenantRoleStore) GetUserTenantRoles(username string) (map[string]inter.TenantRole, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.roles == nil {
		return nil, nil
	}
	return s.roles[username], nil
}

type stubSessionUser struct {
	username string
	perm     inter.PermissionType
}

func (u stubSessionUser) GetPID() string                      { return u.username }
func (u stubSessionUser) GetEmail() string                    { return u.username + "@test.local" }
func (u stubSessionUser) GetUsername() string                 { return u.username }
func (u stubSessionUser) GetPermission() inter.PermissionType { return u.perm }

func TestTenantPrincipalResolverMemberUsesRequestedTenant(t *testing.T) {
	resolver := NewTenantPrincipalResolver(stubTenantRoleStore{
		roles: map[string]map[string]inter.TenantRole{
			"member": {
				"tenant_a": inter.TenantRoleRW,
				"tenant_b": inter.TenantRoleRO,
			},
		},
	})

	principal, err := resolver.Resolve(context.Background(), stubSessionUser{
		username: "member",
		perm:     inter.PermissionReadWrite,
	}, "tenant_b")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if principal.Scope.TenantID != "tenant_b" {
		t.Fatalf("unexpected tenant id: got=%s want=%s", principal.Scope.TenantID, "tenant_b")
	}
	if principal.TenantRoles["tenant_a"] != inter.TenantRoleRW {
		t.Fatalf("unexpected tenant roles: %+v", principal.TenantRoles)
	}
}

func TestTenantPrincipalResolverRejectsCrossTenantRequest(t *testing.T) {
	resolver := NewTenantPrincipalResolver(stubTenantRoleStore{
		roles: map[string]map[string]inter.TenantRole{
			"member": {
				"tenant_a": inter.TenantRoleRW,
			},
		},
	})

	_, err := resolver.Resolve(context.Background(), stubSessionUser{
		username: "member",
		perm:     inter.PermissionReadWrite,
	}, "tenant_b")
	if err == nil {
		t.Fatal("expected cross-tenant request to fail")
	}
	if err != inter.ErrCrossTenantScope {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTenantPrincipalResolverAdminCanSwitchTenant(t *testing.T) {
	resolver := NewTenantPrincipalResolver(stubTenantRoleStore{
		roles: map[string]map[string]inter.TenantRole{
			"admin": {
				"tenant_a": inter.TenantRoleAdmin,
			},
		},
	})

	principal, err := resolver.Resolve(context.Background(), stubSessionUser{
		username: "admin",
		perm:     inter.PermissionAdmin,
	}, "tenant_other")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if principal.Scope.TenantID != "tenant_other" {
		t.Fatalf("unexpected admin tenant id: got=%s want=%s", principal.Scope.TenantID, "tenant_other")
	}
}

func TestTenantPrincipalResolverFallsBackToLegacyTenant(t *testing.T) {
	resolver := NewTenantPrincipalResolver(stubTenantRoleStore{
		roles: map[string]map[string]inter.TenantRole{
			"member": {
				"tenant_legacy": inter.TenantRoleRO,
			},
		},
	})

	principal, err := resolver.Resolve(context.Background(), stubSessionUser{
		username: "member",
		perm:     inter.PermissionReadOnly,
	}, "")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if principal.Scope.TenantID != "tenant_legacy" {
		t.Fatalf("unexpected default tenant id: got=%s want=%s", principal.Scope.TenantID, "tenant_legacy")
	}
}
