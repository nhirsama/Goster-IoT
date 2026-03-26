package tenant_test

import (
	"context"
	"testing"

	identitycore "github.com/nhirsama/Goster-IoT/src/identity"
	"github.com/nhirsama/Goster-IoT/src/inter"
	storageidentity "github.com/nhirsama/Goster-IoT/src/storage/identity"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/testhelper"
	"github.com/nhirsama/Goster-IoT/src/storage/tenant"
	"github.com/nhirsama/Goster-IoT/src/storage/user"
)

func TestRepositoryGetUserTenantRoles(t *testing.T) {
	base, dbPath := testhelper.OpenSQLiteStore(t, "tenant_repo.db")
	authRepo, err := storageidentity.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite auth repo failed: %v", err)
	}
	t.Cleanup(func() {
		_ = authRepo.Close()
	})

	userRepo := user.NewRepository(base.DB)
	tenantRepo := tenant.NewRepository(base.DB)

	if err := authRepo.Create(context.Background(), &identitycore.AuthUser{
		Username: "tenant-member",
		Password: "plain_pw_for_tests",
	}); err != nil {
		t.Fatalf("Create(auth user) failed: %v", err)
	}
	if err := userRepo.UpdateUserPermission("tenant-member", inter.PermissionReadWrite); err != nil {
		t.Fatalf("UpdateUserPermission failed: %v", err)
	}

	roles, err := tenantRepo.GetUserTenantRoles("tenant-member")
	if err != nil {
		t.Fatalf("GetUserTenantRoles failed: %v", err)
	}
	if roles["tenant_legacy"] != inter.TenantRoleRW {
		t.Fatalf("unexpected tenant roles: %+v", roles)
	}
}
