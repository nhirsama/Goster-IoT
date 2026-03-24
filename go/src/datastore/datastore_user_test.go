package datastore

import (
	"context"
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestUserPermissionManagement(t *testing.T) {
	store := newTestStore(t)
	sqlStore := asSQLStore(t, store)

	username := "user_" + randomString(8)
	_, err := sqlStore.db.Exec(
		"INSERT INTO users (email, username, password, permission) VALUES (?, ?, ?, ?)",
		username+"@example.com", username, "hashed-password", inter.PermissionReadOnly,
	)
	if err != nil {
		t.Fatalf("seed users table failed: %v", err)
	}

	count, err := store.GetUserCount()
	if err != nil {
		t.Fatalf("GetUserCount failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("GetUserCount mismatch: got=%d want=1", count)
	}

	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 1 || users[0].Username != username {
		t.Fatalf("ListUsers mismatch: %+v", users)
	}

	perm, err := store.GetUserPermission(username)
	if err != nil {
		t.Fatalf("GetUserPermission failed: %v", err)
	}
	if perm != inter.PermissionReadOnly {
		t.Fatalf("GetUserPermission mismatch: got=%v want=%v", perm, inter.PermissionReadOnly)
	}

	if err := store.UpdateUserPermission(username, inter.PermissionAdmin); err != nil {
		t.Fatalf("UpdateUserPermission failed: %v", err)
	}
	perm, err = store.GetUserPermission(username)
	if err != nil {
		t.Fatalf("GetUserPermission after update failed: %v", err)
	}
	if perm != inter.PermissionAdmin {
		t.Fatalf("permission not updated: got=%v want=%v", perm, inter.PermissionAdmin)
	}
}

func TestUserPermissionNotFound(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.GetUserPermission("missing-user"); err == nil {
		t.Fatal("expected GetUserPermission to fail for missing user")
	}
	if err := store.UpdateUserPermission("missing-user", inter.PermissionAdmin); err == nil {
		t.Fatal("expected UpdateUserPermission to fail for missing user")
	}
}

func TestUserTenantRolesFollowLegacyPermissionMapping(t *testing.T) {
	store := newTestStore(t)
	sqlStore := asSQLStore(t, store)

	if err := sqlStore.Create(context.Background(), &AuthUser{
		Username: "bootstrap_admin",
		Password: "bootstrap-password",
	}); err != nil {
		t.Fatalf("bootstrap create failed: %v", err)
	}
	if err := sqlStore.Create(context.Background(), &AuthUser{
		Username: "member",
		Password: "member-password",
	}); err != nil {
		t.Fatalf("member create failed: %v", err)
	}

	memberRoles, err := store.GetUserTenantRoles("member")
	if err != nil {
		t.Fatalf("GetUserTenantRoles failed: %v", err)
	}
	if got := memberRoles["tenant_legacy"]; got != inter.TenantRoleRO {
		t.Fatalf("unexpected initial tenant role: got=%s want=%s", got, inter.TenantRoleRO)
	}

	if err := store.UpdateUserPermission("member", inter.PermissionReadWrite); err != nil {
		t.Fatalf("UpdateUserPermission failed: %v", err)
	}
	memberRoles, err = store.GetUserTenantRoles("member")
	if err != nil {
		t.Fatalf("GetUserTenantRoles after update failed: %v", err)
	}
	if got := memberRoles["tenant_legacy"]; got != inter.TenantRoleRW {
		t.Fatalf("unexpected updated tenant role: got=%s want=%s", got, inter.TenantRoleRW)
	}

	adminRoles, err := store.GetUserTenantRoles("bootstrap_admin")
	if err != nil {
		t.Fatalf("GetUserTenantRoles bootstrap admin failed: %v", err)
	}
	if got := adminRoles["tenant_legacy"]; got != inter.TenantRoleAdmin {
		t.Fatalf("unexpected bootstrap admin role: got=%s want=%s", got, inter.TenantRoleAdmin)
	}
}
