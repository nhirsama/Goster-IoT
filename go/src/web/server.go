package web

import (
	"errors"
	"net/http"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

type webServer struct {
	dataStore     inter.DataStore
	deviceManager inter.DeviceManager
	api           inter.Api
	auth          AuthService
	captcha       CaptchaVerifier
	logger        inter.Logger
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
	}
	if ws.auth == nil {
		return nil, errors.New("auth service is required")
	}
	return ws, nil
}

// Start 启动标准 HTTP 服务器
func (ws *webServer) Start() {
	addr := ":8080"

	mux := http.NewServeMux()
	ws.registerRoutes(mux)

	ws.logger.Info("web server started", inter.String("addr", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		ws.logger.Error("web server listen failed", inter.Err(err))
		panic(err)
	}
}
