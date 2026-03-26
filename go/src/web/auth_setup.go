package web

import (
	"errors"

	"github.com/aarondl/authboss/v3"
	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/identity"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

// SetupAuthboss 保留兼容入口，内部转发到 identity.SetupAuthboss。
func SetupAuthboss(db inter.DataStore) (*authboss.Authboss, error) {
	store, ok := db.(identity.Store)
	if !ok {
		return nil, errors.New("认证存储未实现 identity.Store 接口")
	}
	return identity.SetupAuthboss(store)
}

// SetupAuthbossWithConfig 保留兼容入口，内部转发到 identity.SetupAuthbossWithConfig。
func SetupAuthbossWithConfig(db inter.DataStore, cfg appcfg.AuthConfig) (*authboss.Authboss, error) {
	store, ok := db.(identity.Store)
	if !ok {
		return nil, errors.New("认证存储未实现 identity.Store 接口")
	}
	return identity.SetupAuthbossWithConfig(store, cfg)
}
