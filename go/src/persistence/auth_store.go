package persistence

import (
	"fmt"
	"strings"

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
		return storageidentity.OpenSQLite(cfg.Path)
	case "postgres":
		if strings.TrimSpace(cfg.DSN) == "" {
			return nil, fmt.Errorf("postgres datastore requires a non-empty dsn")
		}
		if cfg.SchemaMode == "bootstrap" {
			if err := EnsureSchema(cfg); err != nil {
				return nil, err
			}
		}
		return storageidentity.OpenPostgres(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported datastore driver: %s", cfg.Driver)
	}
}
