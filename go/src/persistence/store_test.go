package persistence

import (
	"context"
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

func TestOpenStoreRejectsEmptyPostgresDSN(t *testing.T) {
	_, err := OpenLegacyStore(appcfg.DBConfig{Driver: "postgres"})
	if err == nil {
		t.Fatal("expected postgres backend to reject empty dsn")
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
