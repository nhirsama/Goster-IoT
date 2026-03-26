package identity

import "github.com/aarondl/authboss/v3"

// Store 是认证链路依赖的最小存储组合。
// 这一层只关心身份凭据、OAuth2 和 remember token，不承担业务仓储职责。
type Store interface {
	authboss.ServerStorer
	authboss.CreatingServerStorer
	authboss.OAuth2ServerStorer
	authboss.RememberingServerStorer
}
