package Web

import (
	"html/template"
	"log"
	"net/http"
	"os"

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
	captcha         CaptchaProvider
	authboss        *authboss.Authboss
}

// NewWebServer 创建一个新的 Web 服务器实例
func NewWebServer(ds inter.DataStore, dm inter.DeviceManager, im inter.IdentityManager, api inter.Api, htmlDir string, ab *authboss.Authboss) inter.WebServer {
	providerType := os.Getenv("CAPTCHA_PROVIDER")
	var provider CaptchaProvider

	if providerType == "turnstile" {
		provider = &CloudflareTurnstile{
			SiteKey:   os.Getenv("CF_SITE_KEY"),
			SecretKey: os.Getenv("CF_SECRET_KEY"),
		}
	} else {
		provider = &LocalCaptcha{}
	}

	return &webServer{
		dataStore:       ds,
		deviceManager:   dm,
		identityManager: im,
		api:             api,
		templates:       loadTemplates(htmlDir),
		htmlDir:         htmlDir,
		captcha:         provider,
		authboss:        ab,
	}
}

// Start 启动标准 HTTP 服务器
func (ws *webServer) Start() {
	addr := ":8080"

	// Use authboss router? Or standard Mux?
	// Authboss provides a router or we mount it.
	// Typically we mount authboss at /auth

	mux := http.NewServeMux()
	ws.registerRoutes(mux)

	log.Printf("正在启动 Web 服务器 (HTTP) 于 %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Web 服务器启动失败: %v", err)
	}
}
