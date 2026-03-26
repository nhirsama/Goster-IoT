package persistence

import (
	"fmt"
	"strings"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

// OpenStore 保留兼容入口，当前等价于 OpenLegacyStore。
// 新代码应优先使用 OpenAuthStore 或 OpenRuntimeStore。
func OpenStore(cfg appcfg.DBConfig) (inter.DataStore, error) {
	return OpenLegacyStore(cfg)
}

// OpenLegacyStore 根据配置打开旧版全量 SQL 仓储。
func OpenLegacyStore(cfg appcfg.DBConfig) (inter.DataStore, error) {
	return openLegacySQLStore(cfg)
}

// OpenSQLite 是 sqlite 的兼容便捷入口，主要给旧测试和开发场景使用。
func OpenSQLite(path string) (inter.DataStore, error) {
	return openLegacySQLStore(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   path,
	})
}

// OpenPostgres 是 postgres 的兼容便捷入口，主要给旧测试和开发验证使用。
func OpenPostgres(dsn string) (inter.DataStore, error) {
	return openLegacySQLStore(appcfg.DBConfig{
		Driver: "postgres",
		DSN:    dsn,
	})
}

func openLegacySQLStore(cfg appcfg.DBConfig) (inter.DataStore, error) {
	cfg = appcfg.NormalizeDBConfig(cfg)

	if cfg.SchemaMode != "managed" {
		if err := EnsureSchema(cfg); err != nil {
			return nil, err
		}
		cfg.SchemaMode = "managed"
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Driver)) {
	case "sqlite":
		return datastore.OpenDataStoreSql(cfg.Path)
	case "postgres":
		if strings.TrimSpace(cfg.DSN) == "" {
			return nil, fmt.Errorf("postgres datastore requires a non-empty dsn")
		}
		return datastore.OpenDataStorePostgres(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported datastore driver: %s", cfg.Driver)
	}
}
