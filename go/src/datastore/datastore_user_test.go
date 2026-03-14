package datastore

import (
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
