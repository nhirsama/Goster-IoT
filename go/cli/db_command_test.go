package cli

import (
	"context"
	"path/filepath"
	"testing"

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

	store, err := persistence.OpenStore(appcfg.DBConfig{
		Driver:     "sqlite",
		Path:       dbPath,
		SchemaMode: "managed",
	})
	if err != nil {
		t.Fatalf("OpenStore(managed sqlite) failed after init: %v", err)
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
