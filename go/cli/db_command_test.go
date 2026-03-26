package cli

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/persistence"
)

func TestRunWithArgsDBInitInitializesManagedSQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "managed_init.db")
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_PATH", dbPath)
	t.Setenv("DB_DSN", "")
	t.Setenv("DB_SCHEMA_MODE", "managed")

	if err := RunWithArgs(context.Background(), []string{"db", "init"}); err != nil {
		t.Fatalf("RunWithArgs(db init) failed: %v", err)
	}

	store, err := persistence.OpenLegacyStore(appcfg.DBConfig{
		Driver:     "sqlite",
		Path:       dbPath,
		SchemaMode: "managed",
	})
	if err != nil {
		t.Fatalf("OpenLegacyStore(managed sqlite) failed after init: %v", err)
	}
	if store == nil {
		t.Fatal("managed sqlite store should not be nil after init")
	}
}

func TestRunWithArgsServeRequiresInitializedSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing.db")
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_PATH", dbPath)
	t.Setenv("DB_DSN", "")
	t.Setenv("DB_SCHEMA_MODE", "managed")

	err := RunWithArgs(context.Background(), []string{"serve"})
	if err == nil {
		t.Fatal("expected serve to fail when schema is not initialized")
	}
}

func TestRunWithArgsServeSupportsBunRuntimeStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "bun_runtime.db")
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_PATH", dbPath)
	t.Setenv("DB_DSN", "")
	t.Setenv("DB_SCHEMA_MODE", "managed")
	t.Setenv("DB_STORE_BACKEND", "bun")
	t.Setenv("WEB_HTTP_ADDR", "127.0.0.1:0")
	t.Setenv("API_TCP_ADDR", "127.0.0.1:0")

	if err := RunWithArgs(context.Background(), []string{"db", "init"}); err != nil {
		t.Fatalf("RunWithArgs(db init) failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	if err := RunWithArgs(ctx, []string{"serve"}); err != nil {
		t.Fatalf("RunWithArgs(serve bun runtime) failed: %v", err)
	}

	store, err := persistence.OpenRuntimeStore(appcfg.DBConfig{
		Driver:       "sqlite",
		Path:         dbPath,
		SchemaMode:   "managed",
		StoreBackend: "bun",
	})
	if err != nil {
		t.Fatalf("OpenRuntimeStore(managed sqlite bun) failed after serve: %v", err)
	}
	if store == nil {
		t.Fatal("bun runtime store should not be nil after serve")
	}
	t.Cleanup(func() {
		_ = persistence.CloseIfPossible(store)
	})
}
