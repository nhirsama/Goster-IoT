package v1

import (
	"context"
	"net/http"

	"github.com/aarondl/authboss/v3"
	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
)

// ContextKey 用于在请求上下文中保存 API 级别的元信息。
type ContextKey string

const (
	ContextRequestID ContextKey = "request_id"
	ContextUsername  ContextKey = "username"
	ContextPerm      ContextKey = "permission"
	ContextTenantID  ContextKey = "tenant_id"
)

// ErrorDetail 描述 v1 接口返回的结构化错误信息。
type ErrorDetail struct {
	Type    string                 `json:"type"`
	Field   string                 `json:"field,omitempty"`
	Reason  string                 `json:"reason,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// Envelope 是 v1 接口统一使用的 JSON 包装结构。
type Envelope struct {
	Code      int          `json:"code"`
	Message   string       `json:"message"`
	RequestID string       `json:"request_id"`
	Data      interface{}  `json:"data,omitempty"`
	Error     *ErrorDetail `json:"error,omitempty"`
	Meta      interface{}  `json:"meta,omitempty"`
}

// AuthService 定义了 v1 API 依赖的认证能力边界。
type AuthService interface {
	LoadClientStateMiddleware(next http.Handler) http.Handler
	CurrentUser(r *http.Request) (authboss.User, error)
	NewAuthableUser(ctx context.Context) (authboss.AuthableUser, error)
	CreateUser(ctx context.Context, user authboss.User) error
	HashPassword(password string) (string, error)
	LoadUser(ctx context.Context, pid string) (authboss.User, error)
	VerifyPassword(user authboss.AuthableUser, password string) error
	FireBefore(event authboss.Event, w http.ResponseWriter, r *http.Request) (bool, error)
	FireAfter(event authboss.Event, w http.ResponseWriter, r *http.Request) (bool, error)
	ClearRememberTokens(ctx context.Context, pid string) error
}

// CaptchaVerifier 抽象了公开认证接口使用的验证码校验能力。
type CaptchaVerifier interface {
	IsEnabled() bool
	PublicSiteKey() string
	VerifyToken(token string, ip string) bool
}

// Deps 汇总了 v1 路由运行所需的依赖项。
type Deps struct {
	DataStore         inter.DataStore
	DeviceRegistry    inter.DeviceRegistry
	DevicePresence    inter.DevicePresence
	DownlinkCommands  inter.DownlinkCommandService
	Auth              AuthService
	Captcha           CaptchaVerifier
	Logger            inter.Logger
	Config            appcfg.WebConfig
	LoginAttemptStore LoginAttemptStore  // 允许装配层替换登录失败状态存储，例如 Redis。
	LoginGuard        *LoginAttemptGuard // 允许测试或高级场景直接注入登录保护器。
}

// API 负责 `/api/v1` 路由下的中间件、分发和处理逻辑。
type API struct {
	dataStore        inter.DataStore
	registry         inter.DeviceRegistry
	presence         inter.DevicePresence
	downlinkCommands inter.DownlinkCommandService
	auth             AuthService
	captcha          CaptchaVerifier
	logger           inter.Logger
	config           appcfg.WebConfig
	tenantAccess     *tenantAccessResolver
	loginGuard       *LoginAttemptGuard
}

// New 根据 web 层注入的依赖构造一组 v1 API 处理器。
func New(deps Deps) *API {
	cfg := appcfg.NormalizeWebConfig(deps.Config)
	api := &API{
		dataStore:        deps.DataStore,
		registry:         deps.DeviceRegistry,
		presence:         deps.DevicePresence,
		downlinkCommands: deps.DownlinkCommands,
		auth:             deps.Auth,
		captcha:          deps.Captcha,
		logger:           deps.Logger,
		config:           cfg,
	}
	api.tenantAccess = newTenantAccessResolver(api.dataStore)
	api.loginGuard = deps.LoginGuard
	if api.loginGuard == nil {
		api.loginGuard = NewLoginAttemptGuardWithStore(cfg.LoginProtection, deps.LoginAttemptStore)
	}
	return api
}

// RegisterRoutes 注册完整的 `/api/v1` 路由集合。
func (api *API) RegisterRoutes(mux *http.ServeMux) {
	public := func(h http.HandlerFunc) http.Handler {
		return api.Middleware(api.auth.LoadClientStateMiddleware(h))
	}
	protected := func(h http.HandlerFunc, minPerm inter.PermissionType) http.Handler {
		return api.Middleware(api.auth.LoadClientStateMiddleware(api.AuthMiddleware(h, minPerm)))
	}

	mux.Handle("/api/v1/auth/captcha/config", public(api.CaptchaConfigHandler))
	mux.Handle("/api/v1/auth/register", public(api.RegisterHandler))
	mux.Handle("/api/v1/auth/login", public(api.LoginHandler))
	mux.Handle("/api/v1/auth/logout", public(api.LogoutHandler))
	mux.Handle("/api/v1/auth/me", protected(api.MeHandler, inter.PermissionNone))

	mux.Handle("/api/v1/devices", protected(api.DevicesHandler, inter.PermissionReadOnly))
	mux.Handle("/api/v1/devices/", protected(api.DeviceByUUIDHandler, inter.PermissionReadOnly))

	mux.Handle("/api/v1/metrics/", protected(api.MetricsHandler, inter.PermissionReadOnly))

	mux.Handle("/api/v1/users", protected(api.UsersHandler, inter.PermissionAdmin))
	mux.Handle("/api/v1/users/", protected(api.UserPermissionHandler, inter.PermissionAdmin))
}

func (api *API) log() inter.Logger {
	if api != nil && api.logger != nil {
		return api.logger
	}
	return logger.Default().With(inter.String("module", "web.v1"))
}

func (api *API) webConfig() appcfg.WebConfig {
	return appcfg.NormalizeWebConfig(api.config)
}

func (api *API) maxAPIBodyBytes() int64 {
	return api.webConfig().MaxAPIBodyBytes
}

func (api *API) deviceListDefaultPageSize() int {
	return api.webConfig().DeviceListPage.DefaultSize
}

func (api *API) deviceListMaxPageSize() int {
	return api.webConfig().DeviceListPage.MaxSize
}

func (api *API) metricsMinValidTimestampMs() int64 {
	return api.webConfig().Metrics.MinValidTimestampMs
}

func (api *API) metricsDefaultRangeLabel() string {
	return api.webConfig().Metrics.DefaultRangeLabel
}
