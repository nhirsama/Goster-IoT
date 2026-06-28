package testhelper

import (
	"path/filepath"
	"testing"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/persistence"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/bunrepo"
)

// OpenSQLiteStore 为 storage 仓储测试准备带 schema 的 sqlite bun 连接。
func OpenSQLiteStore(t testing.TB, filename string) (*bunrepo.Store, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), filename)
	if err := persistence.EnsureSchema(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   dbPath,
	}); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	store, err := bunrepo.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("bunrepo.OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store, dbPath
}
