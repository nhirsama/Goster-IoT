package persistence

import (
	"fmt"
	"strings"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

// OpenStore 根据配置打开持久化后端。
// 当前已实现 sqlite；postgres 配置入口已预留，后续可直接在这里接入。
func OpenStore(cfg appcfg.DBConfig) (inter.DataStore, error) {
	cfg = appcfg.NormalizeDBConfig(cfg)

	switch strings.ToLower(strings.TrimSpace(cfg.Driver)) {
	case "sqlite":
		return datastore.NewDataStoreSql(cfg.Path)
	case "postgres":
		return nil, fmt.Errorf("postgres datastore is not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported datastore driver: %s", cfg.Driver)
	}
}

// OpenSQLite 是 sqlite 的便捷入口，主要给测试和开发场景使用。
func OpenSQLite(path string) (inter.DataStore, error) {
	return OpenStore(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   path,
	})
}
