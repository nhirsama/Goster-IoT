package persistence

import (
	"path/filepath"
	"strings"
	"testing"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
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

func TestOpenStoreRejectsUnimplementedPostgres(t *testing.T) {
	_, err := OpenStore(appcfg.DBConfig{
		Driver: "postgres",
		DSN:    "postgres://iot:iot@localhost:5432/goster?sslmode=disable",
	})
	if err == nil {
		t.Fatal("expected postgres backend to be reported as unimplemented")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("unexpected postgres error: %v", err)
	}
}
