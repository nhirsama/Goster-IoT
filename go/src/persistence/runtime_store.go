package persistence

import (
	"fmt"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	storageruntime "github.com/nhirsama/Goster-IoT/src/storage/runtime"
)

// OpenRuntimeStore 打开业务运行时仓储。
// 这里统一协调 schema 初始化模式，并直接走当前标准运行时驱动链。
func OpenRuntimeStore(cfg appcfg.DBConfig) (RuntimeStore, error) {
	cfg = appcfg.NormalizeDBConfig(cfg)

	if cfg.SchemaMode == "bootstrap" {
		if err := EnsureSchema(cfg); err != nil {
			return nil, err
		}
		cfg.SchemaMode = "managed"
	}
	return openBunRuntimeStore(cfg)
}

func openBunRuntimeStore(cfg appcfg.DBConfig) (RuntimeStore, error) {
	switch cfg.Driver {
	case "sqlite":
		store, err := storageruntime.OpenSQLite(cfg.Path)
		if err != nil {
			return nil, err
		}
		if err := validateRuntimeStore(store); err != nil {
			_ = CloseIfPossible(store)
			return nil, err
		}
		return store, nil
	case "postgres":
		if cfg.DSN == "" {
			return nil, fmt.Errorf("postgres driver requires a non-empty dsn")
		}
		store, err := storageruntime.OpenPostgres(cfg.DSN)
		if err != nil {
			return nil, err
		}
		if err := validateRuntimeStore(store); err != nil {
			_ = CloseIfPossible(store)
			return nil, err
		}
		return store, nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
}

func validateRuntimeStore(store RuntimeStore) error {
	if store == nil {
		return fmt.Errorf("runtime store is nil")
	}
	if _, err := store.ListDevices(1, 1); err != nil {
		return err
	}
	if _, err := store.GetUserCount(); err != nil {
		return err
	}
	return nil
}
