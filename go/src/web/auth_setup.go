package web

import (
	"github.com/aarondl/authboss/v3"
	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/identity"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

// SetupAuthboss 保留兼容入口，内部转发到 identity.SetupAuthboss。
func SetupAuthboss(db inter.DataStore) (*authboss.Authboss, error) {
	return identity.SetupAuthboss(db)
}

// SetupAuthbossWithConfig 保留兼容入口，内部转发到 identity.SetupAuthbossWithConfig。
func SetupAuthbossWithConfig(db inter.DataStore, cfg appcfg.AuthConfig) (*authboss.Authboss, error) {
	return identity.SetupAuthbossWithConfig(db, cfg)
}
