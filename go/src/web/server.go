package web

import (
	"html/template"
	"log"
	"net/http"

	"github.com/aarondl/authboss/v3"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

type webServer struct {
	dataStore     inter.DataStore
	deviceManager inter.DeviceManager
	api           inter.Api
	templates     map[string]*template.Template
	htmlDir       string
	authboss      *authboss.Authboss
	turnstile     *TurnstileService
}

// NewWebServer 创建一个新的 web 服务器实例
func NewWebServer(ds inter.DataStore, dm inter.DeviceManager, api inter.Api, htmlDir string, ab *authboss.Authboss) inter.WebServer {
	return &webServer{
		dataStore:     ds,
		deviceManager: dm,
		api:           api,
		templates:     loadTemplates(htmlDir),
		htmlDir:       htmlDir,
		authboss:      ab,
		turnstile:     NewTurnstileService(),
	}
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
