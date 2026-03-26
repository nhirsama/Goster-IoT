package persistence

import (
	"fmt"
	"strings"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	storageruntime "github.com/nhirsama/Goster-IoT/src/storage/runtime"
)

// OpenRuntimeStore 打开业务运行时仓储。
// 这里统一协调 schema 初始化模式与后端选择。
func OpenRuntimeStore(cfg appcfg.DBConfig) (RuntimeStore, error) {
	cfg = appcfg.NormalizeDBConfig(cfg)

	if cfg.SchemaMode == "bootstrap" {
		if err := EnsureSchema(cfg); err != nil {
			return nil, err
		}
		cfg.SchemaMode = "managed"
	}

	switch cfg.StoreBackend {
	case "bun":
		return openBunRuntimeStore(cfg)
	case "sql":
		return openLegacySQLStore(cfg)
	default:
		return nil, fmt.Errorf("unsupported runtime store backend: %s", cfg.StoreBackend)
	}
}

func openBunRuntimeStore(cfg appcfg.DBConfig) (RuntimeStore, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Driver)) {
	case "sqlite":
		return storageruntime.OpenSQLite(cfg.Path)
	case "postgres":
		if strings.TrimSpace(cfg.DSN) == "" {
			return nil, fmt.Errorf("postgres datastore requires a non-empty dsn")
		}
		return storageruntime.OpenPostgres(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported datastore driver: %s", cfg.Driver)
	}
}
