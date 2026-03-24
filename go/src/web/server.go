package web

import (
	"errors"
	"net/http"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
	apiv1 "github.com/nhirsama/Goster-IoT/src/web/v1"
)

type webServer struct {
	dataStore     inter.DataStore
	deviceManager inter.DeviceManager
	api           inter.Api
	auth          AuthService
	captcha       CaptchaVerifier
	loginGuard    *apiv1.LoginAttemptGuard
	apiV1         *apiv1.API
	logger        inter.Logger
	config        appcfg.WebConfig
}

func NewWebServer(deps WebServerDeps) (inter.WebServer, error) {
	ws, err := newWebServer(deps)
	if err != nil {
		return nil, err
	}
	return ws, nil
}

func newWebServer(deps WebServerDeps) (*webServer, error) {
	if err := deps.normalize(); err != nil {
		return nil, err
	}
	ws := &webServer{
		dataStore:     deps.DataStore,
		deviceManager: deps.DeviceManager,
		api:           deps.API,
		auth:          deps.Auth,
		captcha:       deps.Captcha,
		logger:        deps.Logger,
		config:        deps.Config,
	}
	if ws.auth == nil {
		return nil, errors.New("auth service is required")
	}
	ws.loginGuard = newLoginAttemptGuard(ws.config.LoginProtection)
	return ws, nil
}

// Start 启动标准 HTTP 服务器
func (ws *webServer) Start() {
	addr := appcfg.NormalizeWebConfig(ws.config).HTTPAddr

	mux := http.NewServeMux()
	ws.registerRoutes(mux)

	ws.log().Info("Web 服务已启动", inter.String("addr", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		ws.log().Error("Web 服务监听失败", inter.Err(err))
		panic(err)
	}
}

func (ws *webServer) log() inter.Logger {
	if ws != nil && ws.logger != nil {
		return ws.logger
	}
	return logger.Default().With(inter.String("module", "web"))
}
