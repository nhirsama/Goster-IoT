package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aarondl/authboss/v3"
	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	identitycore "github.com/nhirsama/Goster-IoT/src/identity"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestOpenStoreDelegatesToLegacyStore(t *testing.T) {
	store, err := OpenStore(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   filepath.Join(t.TempDir(), "compat.db"),
	})
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	if store == nil {
		t.Fatal("OpenStore should return a store")
	}
}

func TestOpenStoreSupportsSQLite(t *testing.T) {
	store, err := OpenLegacyStore(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   filepath.Join(t.TempDir(), "persistence.db"),
	})
	if err != nil {
		t.Fatalf("OpenLegacyStore(sqlite) failed: %v", err)
	}
	if store == nil {
		t.Fatal("sqlite store should not be nil")
	}
}

func TestOpenSQLiteWrapperSupportsSQLite(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "sqlite-wrapper.db"))
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	if store == nil {
		t.Fatal("OpenSQLite should return a store")
	}
}

func TestOpenStoreRejectsManagedSQLiteWithoutSchema(t *testing.T) {
	_, err := OpenLegacyStore(appcfg.DBConfig{
		Driver:     "sqlite",
		Path:       filepath.Join(t.TempDir(), "missing.db"),
		SchemaMode: "managed",
	})
	if err == nil {
		t.Fatal("expected managed sqlite to fail before schema init")
	}
}

func TestOpenStoreSupportsManagedSQLiteAfterExplicitEnsure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "managed.db")
	if err := EnsureSchema(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   dbPath,
	}); err != nil {
		t.Fatalf("EnsureSchema(sqlite) failed: %v", err)
	}

	store, err := OpenLegacyStore(appcfg.DBConfig{
		Driver:     "sqlite",
		Path:       dbPath,
		SchemaMode: "managed",
	})
	if err != nil {
		t.Fatalf("OpenLegacyStore(sqlite managed) failed: %v", err)
	}
	if store == nil {
		t.Fatal("managed sqlite store should not be nil")
	}
}

func TestOpenRuntimeStoreSupportsSQLManagedSQLiteAfterExplicitEnsure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "managed_sql.db")
	if err := EnsureSchema(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   dbPath,
	}); err != nil {
		t.Fatalf("EnsureSchema(sqlite) failed: %v", err)
	}

	store, err := OpenRuntimeStore(appcfg.DBConfig{
		Driver:       "sqlite",
		Path:         dbPath,
		SchemaMode:   "managed",
		StoreBackend: "sql",
	})
	if err != nil {
		t.Fatalf("OpenRuntimeStore(sqlite sql) failed: %v", err)
	}
	if store == nil {
		t.Fatal("managed sql runtime store should not be nil")
	}
	t.Cleanup(func() {
		_ = CloseIfPossible(store)
	})
}

func TestOpenRuntimeStoreSupportsBunBootstrapSQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "bootstrap_bun.db")
	store, err := OpenRuntimeStore(appcfg.DBConfig{
		Driver:       "sqlite",
		Path:         dbPath,
		SchemaMode:   "bootstrap",
		StoreBackend: "bun",
	})
	if err != nil {
		t.Fatalf("OpenRuntimeStore(sqlite bun bootstrap) failed: %v", err)
	}
	t.Cleanup(func() {
		_ = CloseIfPossible(store)
	})

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected sqlite db file to exist after bootstrap: %v", err)
	}
}

func TestOpenRuntimeStoreSupportsBunManagedSQLiteAfterExplicitEnsure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "managed_bun.db")
	if err := EnsureSchema(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   dbPath,
	}); err != nil {
		t.Fatalf("EnsureSchema(sqlite) failed: %v", err)
	}

	store, err := OpenRuntimeStore(appcfg.DBConfig{
		Driver:       "sqlite",
		Path:         dbPath,
		SchemaMode:   "managed",
		StoreBackend: "bun",
	})
	if err != nil {
		t.Fatalf("OpenRuntimeStore(sqlite bun) failed: %v", err)
	}
	if store == nil {
		t.Fatal("managed bun runtime store should not be nil")
	}
	t.Cleanup(func() {
		_ = CloseIfPossible(store)
	})

	deviceID := fmt.Sprintf("sqlite-bun-device-%d", time.Now().UnixNano())
	token := fmt.Sprintf("sqlite-bun-token-%d", time.Now().UnixNano())
	meta := inter.DeviceMetadata{
		Name:               "sqlite-bun-smoke",
		Token:              token,
		AuthenticateStatus: inter.AuthenticatePending,
	}
	if err := store.InitDevice(deviceID, meta); err != nil {
		t.Fatalf("InitDevice(sqlite bun) failed: %v", err)
	}
	loaded, err := store.LoadConfig(deviceID)
	if err != nil {
		t.Fatalf("LoadConfig(sqlite bun) failed: %v", err)
	}
	if loaded.Name != meta.Name || loaded.Token != meta.Token {
		t.Fatalf("unexpected bun sqlite metadata: %+v", loaded)
	}
}

func TestOpenRuntimeStoreFallsBackToDefaultBackend(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fallback_backend.db")
	store, err := OpenRuntimeStore(appcfg.DBConfig{
		Driver:       "sqlite",
		Path:         dbPath,
		SchemaMode:   "bootstrap",
		StoreBackend: "bad",
	})
	if err != nil {
		t.Fatalf("expected fallback backend to work, got: %v", err)
	}
	if store == nil {
		t.Fatal("fallback backend store should not be nil")
	}
	t.Cleanup(func() {
		_ = CloseIfPossible(store)
	})
}

func TestOpenRuntimeStoreFallsBackToSQLiteDriver(t *testing.T) {
	store, err := OpenRuntimeStore(appcfg.DBConfig{
		Driver:       "mysql",
		Path:         filepath.Join(t.TempDir(), "fallback_driver.db"),
		SchemaMode:   "bootstrap",
		StoreBackend: "bun",
	})
	if err != nil {
		t.Fatalf("expected fallback sqlite driver to work, got: %v", err)
	}
	if store == nil {
		t.Fatal("fallback driver store should not be nil")
	}
	t.Cleanup(func() {
		_ = CloseIfPossible(store)
	})
}

func TestOpenAuthStoreSupportsRememberingSQLiteAfterExplicitEnsure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "managed_auth.db")
	if err := EnsureSchema(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   dbPath,
	}); err != nil {
		t.Fatalf("EnsureSchema(sqlite) failed: %v", err)
	}

	store, err := OpenAuthStore(appcfg.DBConfig{
		Driver:     "sqlite",
		Path:       dbPath,
		SchemaMode: "managed",
	})
	if err != nil {
		t.Fatalf("OpenAuthStore(sqlite managed) failed: %v", err)
	}
	t.Cleanup(func() {
		_ = CloseIfPossible(store)
	})

	creator, ok := store.(authboss.CreatingServerStorer)
	if !ok {
		t.Fatal("auth store should implement CreatingServerStorer")
	}
	if err := creator.Create(context.Background(), &identitycore.AuthUser{
		Username: "remember-factory-user",
		Password: "plain_pw_for_tests",
	}); err != nil {
		t.Fatalf("Create(auth store) failed: %v", err)
	}

	remembering, ok := store.(authboss.RememberingServerStorer)
	if !ok {
		t.Fatal("auth store should implement RememberingServerStorer")
	}
	if err := remembering.AddRememberToken(context.Background(), "remember-factory-user", "tok-factory"); err != nil {
		t.Fatalf("AddRememberToken(auth store) failed: %v", err)
	}
}

func TestOpenAuthStoreSupportsBootstrapSQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "bootstrap_auth.db")
	store, err := OpenAuthStore(appcfg.DBConfig{
		Driver:     "sqlite",
		Path:       dbPath,
		SchemaMode: "bootstrap",
	})
	if err != nil {
		t.Fatalf("OpenAuthStore(sqlite bootstrap) failed: %v", err)
	}
	t.Cleanup(func() {
		_ = CloseIfPossible(store)
	})
}

func TestOpenAuthStoreFallsBackToSQLiteDriver(t *testing.T) {
	store, err := OpenAuthStore(appcfg.DBConfig{
		Driver:     "mysql",
		Path:       filepath.Join(t.TempDir(), "fallback_auth.db"),
		SchemaMode: "bootstrap",
	})
	if err != nil {
		t.Fatalf("expected fallback sqlite auth store to work, got: %v", err)
	}
	t.Cleanup(func() {
		_ = CloseIfPossible(store)
	})
}

func TestEnsureSchemaFallsBackToSQLiteDriver(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fallback_schema.db")
	if err := EnsureSchema(appcfg.DBConfig{Driver: "mysql", Path: dbPath}); err != nil {
		t.Fatalf("expected fallback sqlite ensure schema to work, got: %v", err)
	}
}

func TestEnsureSchemaRejectsEmptyPostgresDSN(t *testing.T) {
	err := EnsureSchema(appcfg.DBConfig{Driver: "postgres"})
	if err == nil {
		t.Fatal("expected postgres dsn error")
	}
}

func TestOpenStoreRejectsEmptyPostgresDSN(t *testing.T) {
	_, err := OpenLegacyStore(appcfg.DBConfig{Driver: "postgres"})
	if err == nil {
		t.Fatal("expected postgres backend to reject empty dsn")
	}
}

func TestOpenPostgresRejectsEmptyDSN(t *testing.T) {
	_, err := OpenPostgres("")
	if err == nil {
		t.Fatal("expected OpenPostgres to reject empty dsn")
	}
}

func TestCloseIfPossibleHandlesNilAndNonCloser(t *testing.T) {
	if err := CloseIfPossible(nil); err != nil {
		t.Fatalf("CloseIfPossible(nil) failed: %v", err)
	}
	if err := CloseIfPossible(struct{}{}); err != nil {
		t.Fatalf("CloseIfPossible(non-closer) failed: %v", err)
	}
}

func TestCloseIfPossiblePropagatesCloserError(t *testing.T) {
	want := errors.New("close failed")
	err := CloseIfPossible(testCloser{err: want})
	if !errors.Is(err, want) {
		t.Fatalf("expected close error %v, got %v", want, err)
	}
}

func TestOpenStoreSupportsPostgresWhenDSNProvided(t *testing.T) {
	dsn := os.Getenv("PG_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_TEST_DSN 未设置，跳过 PostgreSQL 集成测试")
	}

	if err := EnsureSchema(appcfg.DBConfig{
		Driver: "postgres",
		DSN:    dsn,
	}); err != nil {
		t.Fatalf("EnsureSchema(postgres) failed: %v", err)
	}

	store, err := OpenLegacyStore(appcfg.DBConfig{
		Driver:     "postgres",
		DSN:        dsn,
		SchemaMode: "managed",
	})
	if err != nil {
		t.Fatalf("OpenLegacyStore(postgres) failed: %v", err)
	}
	if store == nil {
		t.Fatal("postgres store should not be nil")
	}

	deviceID := fmt.Sprintf("pg-smoke-device-%d", time.Now().UnixNano())
	token := fmt.Sprintf("pg-smoke-token-%d", time.Now().UnixNano())
	meta := inter.DeviceMetadata{
		Name:               "pg-smoke",
		Token:              token,
		AuthenticateStatus: inter.AuthenticatePending,
	}
	if err := store.InitDevice(deviceID, meta); err != nil {
		t.Fatalf("InitDevice(postgres) failed: %v", err)
	}
	loaded, err := store.LoadConfig(deviceID)
	if err != nil {
		t.Fatalf("LoadConfig(postgres) failed: %v", err)
	}
	if loaded.Name != meta.Name || loaded.Token != meta.Token {
		t.Fatalf("unexpected postgres metadata: %+v", loaded)
	}
}

func TestOpenRuntimeStoreSupportsPostgresBunWhenDSNProvided(t *testing.T) {
	dsn := os.Getenv("PG_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_TEST_DSN 未设置，跳过 PostgreSQL 集成测试")
	}

	if err := EnsureSchema(appcfg.DBConfig{
		Driver: "postgres",
		DSN:    dsn,
	}); err != nil {
		t.Fatalf("EnsureSchema(postgres) failed: %v", err)
	}

	store, err := OpenRuntimeStore(appcfg.DBConfig{
		Driver:       "postgres",
		DSN:          dsn,
		SchemaMode:   "managed",
		StoreBackend: "bun",
	})
	if err != nil {
		t.Fatalf("OpenRuntimeStore(postgres bun) failed: %v", err)
	}
	if store == nil {
		t.Fatal("postgres bun runtime store should not be nil")
	}
	t.Cleanup(func() {
		_ = CloseIfPossible(store)
	})

	deviceID := fmt.Sprintf("pg-bun-runtime-device-%d", time.Now().UnixNano())
	token := fmt.Sprintf("pg-bun-runtime-token-%d", time.Now().UnixNano())
	meta := inter.DeviceMetadata{
		Name:               "pg-bun-runtime",
		Token:              token,
		AuthenticateStatus: inter.AuthenticatePending,
	}
	if err := store.InitDevice(deviceID, meta); err != nil {
		t.Fatalf("InitDevice(postgres bun) failed: %v", err)
	}
	loaded, err := store.LoadConfig(deviceID)
	if err != nil {
		t.Fatalf("LoadConfig(postgres bun) failed: %v", err)
	}
	if loaded.Name != meta.Name || loaded.Token != meta.Token {
		t.Fatalf("unexpected postgres bun metadata: %+v", loaded)
	}
}

type testCloser struct {
	err error
}

func (c testCloser) Close() error {
	return c.err
}
