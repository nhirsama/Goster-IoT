package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestOpenStoreSupportsSQLite(t *testing.T) {
	store, err := OpenStore(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   filepath.Join(t.TempDir(), "persistence.db"),
	})
	if err != nil {
		t.Fatalf("OpenStore(sqlite) failed: %v", err)
	}
	if store == nil {
		t.Fatal("sqlite store should not be nil")
	}
}

func TestOpenStoreRejectsManagedSQLiteWithoutSchema(t *testing.T) {
	_, err := OpenStore(appcfg.DBConfig{
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

	store, err := OpenStore(appcfg.DBConfig{
		Driver:     "sqlite",
		Path:       dbPath,
		SchemaMode: "managed",
	})
	if err != nil {
		t.Fatalf("OpenStore(sqlite managed) failed: %v", err)
	}
	if store == nil {
		t.Fatal("managed sqlite store should not be nil")
	}
}

func TestOpenStoreRejectsEmptyPostgresDSN(t *testing.T) {
	_, err := OpenStore(appcfg.DBConfig{Driver: "postgres"})
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

	store, err := OpenStore(appcfg.DBConfig{
		Driver:     "postgres",
		DSN:        dsn,
		SchemaMode: "managed",
	})
	if err != nil {
		t.Fatalf("OpenStore(postgres) failed: %v", err)
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
