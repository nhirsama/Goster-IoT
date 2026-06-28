package identity_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/aarondl/authboss/v3"
	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	identitypkg "github.com/nhirsama/Goster-IoT/src/identity"
	"github.com/nhirsama/Goster-IoT/src/persistence"
)

func openSQLiteAuthStoreForIdentity(t *testing.T) identitypkg.Store {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "identity.db")
	if err := persistence.EnsureSchema(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   dbPath,
	}); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	store, err := persistence.OpenAuthStore(appcfg.DBConfig{
		Driver:     "sqlite",
		Path:       dbPath,
		SchemaMode: "managed",
	})
	if err != nil {
		t.Fatalf("OpenAuthStore failed: %v", err)
	}
	t.Cleanup(func() {
		_ = persistence.CloseIfPossible(store)
	})
	return store
}

func TestSetupAuthbossWithConfigAndService(t *testing.T) {
	store := openSQLiteAuthStoreForIdentity(t)
	cfg := appcfg.DefaultAuthConfig()
	cfg.RootURL = "http://example.test"
	cfg.CookieSecure = false

	ab, err := identitypkg.SetupAuthbossWithConfig(store, cfg)
	if err != nil {
		t.Fatalf("SetupAuthbossWithConfig failed: %v", err)
	}
	if ab.Config.Paths.RootURL != cfg.RootURL {
		t.Fatalf("unexpected root url: %s", ab.Config.Paths.RootURL)
	}
	if ab.Config.Storage.Server == nil || ab.Config.Storage.SessionState == nil || ab.Config.Storage.CookieState == nil {
		t.Fatal("authboss storage should be initialized")
	}

	service, err := identitypkg.NewAuthbossService(ab)
	if err != nil {
		t.Fatalf("NewAuthbossService failed: %v", err)
	}
	authable, err := service.NewAuthableUser(context.Background())
	if err != nil {
		t.Fatalf("NewAuthableUser failed: %v", err)
	}

	hashed, err := service.HashPassword("secret-password")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	user := authable.(*identitypkg.AuthUser)
	user.PutPID("auth-user")
	user.PutPassword(hashed)
	if err := service.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	loaded, err := service.LoadUser(context.Background(), "auth-user")
	if err != nil {
		t.Fatalf("LoadUser failed: %v", err)
	}
	if err := service.VerifyPassword(loaded.(authboss.AuthableUser), "secret-password"); err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if err := service.ClearRememberTokens(context.Background(), "auth-user"); err != nil {
		t.Fatalf("ClearRememberTokens failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := service.CurrentUser(req); err == nil {
		t.Fatal("CurrentUser should fail without session")
	}

	called := false
	service.LoadClientStateMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !called {
		t.Fatal("LoadClientStateMiddleware should call next")
	}

	handled, err := service.FireBefore(authboss.EventAuth, httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil || handled {
		t.Fatalf("FireBefore unexpected result: handled=%v err=%v", handled, err)
	}
	handled, err = service.FireAfter(authboss.EventAuth, httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil || handled {
		t.Fatalf("FireAfter unexpected result: handled=%v err=%v", handled, err)
	}
}

func TestNewAuthbossServiceRejectsNil(t *testing.T) {
	if _, err := identitypkg.NewAuthbossService(nil); err == nil {
		t.Fatal("expected nil authboss instance to fail")
	}
}
