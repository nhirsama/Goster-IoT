package persistence

import (
	"fmt"
	"strings"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

// OpenStore 根据配置打开持久化后端。
// schema 初始化策略由 DBConfig.SchemaMode 决定：
// 1. bootstrap: 显式执行建表与兼容迁移，再打开存储。
// 2. managed: 只打开现有数据库，要求外部迁移工具已完成建库。
func OpenStore(cfg appcfg.DBConfig) (inter.DataStore, error) {
	cfg = appcfg.NormalizeDBConfig(cfg)

	switch strings.ToLower(strings.TrimSpace(cfg.Driver)) {
	case "sqlite":
		if cfg.SchemaMode == "managed" {
			return datastore.OpenDataStoreSql(cfg.Path)
		}
		return datastore.NewDataStoreSql(cfg.Path)
	case "postgres":
		if strings.TrimSpace(cfg.DSN) == "" {
			return nil, fmt.Errorf("postgres datastore requires a non-empty dsn")
		}
		if cfg.SchemaMode == "managed" {
			return datastore.OpenDataStorePostgres(cfg.DSN)
		}
		return datastore.NewDataStorePostgres(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported datastore driver: %s", cfg.Driver)
	}
}

// EnsureSchema 显式初始化数据库表结构。
// 这个入口是未来接 Atlas / 独立 init 命令时的过渡边界。
func EnsureSchema(cfg appcfg.DBConfig) error {
	cfg = appcfg.NormalizeDBConfig(cfg)

	switch cfg.Driver {
	case "sqlite":
		return datastore.EnsureSQLiteSchema(cfg.Path)
	case "postgres":
		if strings.TrimSpace(cfg.DSN) == "" {
			return fmt.Errorf("postgres datastore requires a non-empty dsn")
		}
		return datastore.EnsurePostgresSchema(cfg.DSN)
	default:
		return fmt.Errorf("unsupported datastore driver: %s", cfg.Driver)
	}
}

// OpenSQLite 是 sqlite 的便捷入口，主要给测试和开发场景使用。
func OpenSQLite(path string) (inter.DataStore, error) {
	return OpenStore(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   path,
	})
}

// OpenPostgres 是 postgres 的便捷入口，主要给开发验证和后续集成测试使用。
func OpenPostgres(dsn string) (inter.DataStore, error) {
	return OpenStore(appcfg.DBConfig{
		Driver: "postgres",
		DSN:    dsn,
	})
}
