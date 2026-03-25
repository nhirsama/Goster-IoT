package web

import (
	"github.com/aarondl/authboss/v3"
	"github.com/nhirsama/Goster-IoT/src/identity"
)

// AuthService 保留为兼容别名，新的代码优先直接依赖 identity.Service。
type AuthService = identity.Service

// NewAuthService 保留兼容入口，内部转发到 identity.NewAuthbossService。
func NewAuthService(ab *authboss.Authboss) (AuthService, error) {
	return identity.NewAuthbossService(ab)
}
