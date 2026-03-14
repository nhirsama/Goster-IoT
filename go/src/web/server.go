package web

import (
	"errors"
	"log"
	"net/http"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

type webServer struct {
	dataStore     inter.DataStore
	deviceManager inter.DeviceManager
	api           inter.Api
	auth          AuthService
	captcha       CaptchaVerifier
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

	log.Printf("正在启动 web 服务器 (HTTP) 于 %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("web 服务器启动失败: %v", err)
	}
}
