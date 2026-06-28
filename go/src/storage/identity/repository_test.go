package identitystore_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/aarondl/authboss/v3"
	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	identitycore "github.com/nhirsama/Goster-IoT/src/identity"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/persistence"
	identitystore "github.com/nhirsama/Goster-IoT/src/storage/identity"
	storageruntime "github.com/nhirsama/Goster-IoT/src/storage/runtime"
)

func openSQLiteAuthRepo(t *testing.T) (*identitystore.Repository, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "auth_store.db")
	if err := persistence.EnsureSchema(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   dbPath,
	}); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	repo, err := identitystore.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite auth repo failed: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Close()
	})
	return repo, dbPath
}

func TestRepositoryCreateLoadAndTenantRoleSync(t *testing.T) {
	repo, dbPath := openSQLiteAuthRepo(t)

	user := &identitycore.AuthUser{
		Username: "member",
		Password: "plain_pw_for_tests",
	}
	if err := repo.Create(context.Background(), user); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	loaded, err := repo.Load(context.Background(), "member")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	got, ok := loaded.(*identitycore.AuthUser)
	if !ok {
		t.Fatalf("unexpected user type: %T", loaded)
	}
	if got.Username != "member" {
		t.Fatalf("unexpected username: %s", got.Username)
	}
	if got.GetPermission() != inter.PermissionAdmin {
		t.Fatalf("first user should be admin, got=%v", got.GetPermission())
	}

	runtimeStore, err := storageruntime.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite runtime store failed: %v", err)
	}
	t.Cleanup(func() {
		_ = runtimeStore.Close()
	})
	roles, err := runtimeStore.GetUserTenantRoles("member")
	if err != nil {
		t.Fatalf("GetUserTenantRoles failed: %v", err)
	}
	if roles[bunLegacyTenantID()] != inter.TenantRoleAdmin {
		t.Fatalf("unexpected tenant role map: %+v", roles)
	}
}

func TestRepositoryRememberTokenLifecycle(t *testing.T) {
	repo, _ := openSQLiteAuthRepo(t)

	user := &identitycore.AuthUser{
		Username: "remember-user",
		Password: "plain_pw_for_tests",
	}
	if err := repo.Create(context.Background(), user); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.AddRememberToken(context.Background(), "remember-user", "tok-1"); err != nil {
		t.Fatalf("AddRememberToken failed: %v", err)
	}

	loaded, err := repo.Load(context.Background(), "remember-user")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	got := loaded.(*identitycore.AuthUser)
	if got.RememberToken != "tok-1" {
		t.Fatalf("unexpected remember token: %q", got.RememberToken)
	}

	if err := repo.UseRememberToken(context.Background(), "remember-user", "tok-1"); err != nil {
		t.Fatalf("UseRememberToken failed: %v", err)
	}
	if err := repo.UseRememberToken(context.Background(), "remember-user", "tok-1"); err != authboss.ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound, got: %v", err)
	}

	if err := repo.AddRememberToken(context.Background(), "remember-user", "tok-2"); err != nil {
		t.Fatalf("AddRememberToken(second) failed: %v", err)
	}
	if err := repo.DelRememberTokens(context.Background(), "remember-user"); err != nil {
		t.Fatalf("DelRememberTokens failed: %v", err)
	}
	loaded, err = repo.Load(context.Background(), "remember-user")
	if err != nil {
		t.Fatalf("Load(after delete) failed: %v", err)
	}
	if loaded.(*identitycore.AuthUser).RememberToken != "" {
		t.Fatalf("remember token should be cleared, got=%q", loaded.(*identitycore.AuthUser).RememberToken)
	}
}

func TestRepositorySaveOAuth2CreatesUser(t *testing.T) {
	repo, _ := openSQLiteAuthRepo(t)

	user := &identitycore.AuthUser{
		OAuth2Provider: "github",
		OAuth2UID:      "42",
		Email:          "octo@example.com",
	}
	if err := repo.SaveOAuth2(context.Background(), user); err != nil {
		t.Fatalf("SaveOAuth2 failed: %v", err)
	}

	loaded, err := repo.Load(context.Background(), "github_42")
	if err != nil {
		t.Fatalf("Load(oauth2 user) failed: %v", err)
	}
	got := loaded.(*identitycore.AuthUser)
	if got.Username != "github_42" {
		t.Fatalf("unexpected oauth username: %s", got.Username)
	}
	if got.Email != "octo@example.com" {
		t.Fatalf("unexpected oauth email: %s", got.Email)
	}
}

func bunLegacyTenantID() string {
	return "tenant_legacy"
}
