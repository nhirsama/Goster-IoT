package user_test

import (
	"context"
	"testing"

	identitycore "github.com/nhirsama/Goster-IoT/src/identity"
	"github.com/nhirsama/Goster-IoT/src/inter"
	storageidentity "github.com/nhirsama/Goster-IoT/src/storage/identity"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/testhelper"
	"github.com/nhirsama/Goster-IoT/src/storage/user"
)

func TestRepositoryUserPermissionAndLegacyTenantSync(t *testing.T) {
	base, dbPath := testhelper.OpenSQLiteStore(t, "user_repo.db")
	authRepo, err := storageidentity.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite auth repo failed: %v", err)
	}
	t.Cleanup(func() {
		_ = authRepo.Close()
	})
	repo := user.NewRepository(base.DB)

	if err := authRepo.Create(context.Background(), &identitycore.AuthUser{
		Username: "member",
		Password: "plain_pw_for_tests",
	}); err != nil {
		t.Fatalf("Create(auth user) failed: %v", err)
	}

	count, err := repo.GetUserCount()
	if err != nil {
		t.Fatalf("GetUserCount failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("unexpected user count: %d", count)
	}

	if err := repo.UpdateUserPermission("member", inter.PermissionReadWrite); err != nil {
		t.Fatalf("UpdateUserPermission failed: %v", err)
	}

	perm, err := repo.GetUserPermission("member")
	if err != nil {
		t.Fatalf("GetUserPermission failed: %v", err)
	}
	if perm != inter.PermissionReadWrite {
		t.Fatalf("unexpected permission: %v", perm)
	}

	users, err := repo.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 1 || users[0].Username != "member" {
		t.Fatalf("unexpected users: %+v", users)
	}
}
