package persistence

import (
	"context"
	"fmt"
	"strings"

	"github.com/aarondl/authboss/v3"
	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	identitycore "github.com/nhirsama/Goster-IoT/src/identity"
	storageidentity "github.com/nhirsama/Goster-IoT/src/storage/identity"
)

// OpenAuthStore 打开认证链路使用的最小存储模块。
// 它只负责身份凭据、OAuth2 和 remember token。
func OpenAuthStore(cfg appcfg.DBConfig) (identitycore.Store, error) {
	cfg = appcfg.NormalizeDBConfig(cfg)

	switch strings.ToLower(strings.TrimSpace(cfg.Driver)) {
	case "sqlite":
		if cfg.SchemaMode == "bootstrap" {
			if err := EnsureSchema(cfg); err != nil {
				return nil, err
			}
		}
		store, err := storageidentity.OpenSQLite(cfg.Path)
		if err != nil {
			return nil, err
		}
		if err := validateAuthStore(store); err != nil {
			_ = CloseIfPossible(store)
			return nil, err
		}
		return store, nil
	case "postgres":
		if strings.TrimSpace(cfg.DSN) == "" {
			return nil, fmt.Errorf("postgres driver requires a non-empty dsn")
		}
		if cfg.SchemaMode == "bootstrap" {
			if err := EnsureSchema(cfg); err != nil {
				return nil, err
			}
		}
		store, err := storageidentity.OpenPostgres(cfg.DSN)
		if err != nil {
			return nil, err
		}
		if err := validateAuthStore(store); err != nil {
			_ = CloseIfPossible(store)
			return nil, err
		}
		return store, nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
}

func validateAuthStore(store identitycore.Store) error {
	if store == nil {
		return fmt.Errorf("auth store is nil")
	}
	_, err := store.Load(context.Background(), "__schema_probe__")
	if err == nil || err == authboss.ErrUserNotFound {
		return nil
	}
	return err
}
