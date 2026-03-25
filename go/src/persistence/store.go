package persistence

import (
	"fmt"
	"strings"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/inter"
	storageruntime "github.com/nhirsama/Goster-IoT/src/storage/runtime"
)

// RuntimeStore 是当前业务运行时依赖的最小仓储组合。
// 认证链路暂时仍保留在旧 datastore 上，因此这里不要求 Authboss 的接口实现。
type RuntimeStore interface {
	inter.CoreStore
	inter.WebV1Store
}

// OpenStore 根据配置打开持久化后端。
// schema 初始化策略由 DBConfig.SchemaMode 决定：
// 1. bootstrap: 显式执行建表与兼容迁移，再打开存储。
// 2. managed: 只打开现有数据库，要求外部迁移工具已完成建库。
func OpenStore(cfg appcfg.DBConfig) (inter.DataStore, error) {
	return OpenAuthStore(cfg)
}

// OpenAuthStore 打开认证链路使用的旧版全量仓储。
// 这里保留 SQL 版本实现，避免在 Authboss 切换完成前扩大迁移面。
func OpenAuthStore(cfg appcfg.DBConfig) (inter.DataStore, error) {
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

// OpenRuntimeStore 打开业务运行时仓储。
// 这条链路支持逐步切换到 bunstore，同时保留 sql 作为回退实现。
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
		return OpenAuthStore(cfg)
	default:
		return nil, fmt.Errorf("unsupported runtime store backend: %s", cfg.StoreBackend)
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
	return OpenAuthStore(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   path,
	})
}

// OpenPostgres 是 postgres 的便捷入口，主要给开发验证和后续集成测试使用。
func OpenPostgres(dsn string) (inter.DataStore, error) {
	return OpenAuthStore(appcfg.DBConfig{
		Driver: "postgres",
		DSN:    dsn,
	})
}

func openBunRuntimeStore(cfg appcfg.DBConfig) (RuntimeStore, error) {
	switch cfg.Driver {
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

// CloseIfPossible 在存储实现提供 Close 方法时执行资源释放。
func CloseIfPossible(store any) error {
	closer, ok := store.(interface{ Close() error })
	if !ok {
		return nil
	}
	return closer.Close()
}
