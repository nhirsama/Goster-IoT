package web

import (
	"context"
	"errors"
	"net/http"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
)

type webServer struct {
	dataStore     inter.DataStore
	deviceManager inter.DeviceManager
	api           inter.Api
	auth          AuthService
	captcha       CaptchaVerifier
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
	return ws, nil
}

// Start 启动标准 HTTP 服务器
func (ws *webServer) Start(ctx context.Context) error {
	addr := appcfg.NormalizeWebConfig(ws.config).HTTPAddr

	mux := http.NewServeMux()
	ws.registerRoutes(mux)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// 启动 goroutine 监听 context 取消信号
	go func() {
		<-ctx.Done()
		ws.log().Info("收到关闭信号，准备关闭 Web 服务")

		// 给予 5 秒优雅关闭时间
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			ws.log().Error("Web 服务关闭失败", inter.Err(err))
		} else {
			ws.log().Info("Web 服务已优雅关闭")
		}
	}()

	ws.log().Info("Web 服务已启动", inter.String("addr", addr))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		ws.log().Error("Web 服务监听失败", inter.Err(err))
		return err
	}
	return nil
}

func (ws *webServer) log() inter.Logger {
	if ws != nil && ws.logger != nil {
		return ws.logger
	}
	return logger.Default().With(inter.String("module", "web"))
}
