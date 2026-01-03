package Web

import (
	"html/template"
	"log"
	"net/http"

	"github.com/aarondl/authboss/v3"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

type webServer struct {
	dataStore       inter.DataStore
	deviceManager   inter.DeviceManager
	identityManager inter.IdentityManager
	api             inter.Api
	templates       map[string]*template.Template
	htmlDir         string
	authboss        *authboss.Authboss
}

// NewWebServer 创建一个新的 Web 服务器实例
func NewWebServer(ds inter.DataStore, dm inter.DeviceManager, im inter.IdentityManager, api inter.Api, htmlDir string, ab *authboss.Authboss) inter.WebServer {
	return &webServer{
		dataStore:       ds,
		deviceManager:   dm,
		identityManager: im,
		api:             api,
		templates:       loadTemplates(htmlDir),
		htmlDir:         htmlDir,
		authboss:        ab,
	}
}

// Start 启动标准 HTTP 服务器
func (ws *webServer) Start() {
	addr := ":8080"

	mux := http.NewServeMux()
	ws.registerRoutes(mux)

	log.Printf("正在启动 Web 服务器 (HTTP) 于 %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Web 服务器启动失败: %v", err)
	}
}
