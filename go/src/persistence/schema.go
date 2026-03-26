package persistence

import (
	"fmt"
	"strings"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/dbschema"
)

// EnsureSchema 显式初始化数据库表结构。
// 这个入口是未来接 Atlas / 独立 init 命令时的统一边界。
func EnsureSchema(cfg appcfg.DBConfig) error {
	cfg = appcfg.NormalizeDBConfig(cfg)

	switch strings.ToLower(strings.TrimSpace(cfg.Driver)) {
	case "sqlite":
		return dbschema.EnsureSQLite(cfg.Path)
	case "postgres":
		if strings.TrimSpace(cfg.DSN) == "" {
			return fmt.Errorf("postgres datastore requires a non-empty dsn")
		}
		return dbschema.EnsurePostgres(cfg.DSN)
	default:
		return fmt.Errorf("unsupported datastore driver: %s", cfg.Driver)
	}
}
