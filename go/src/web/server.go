package web

import (
	"errors"
	"net/http"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
)

type webServer struct {
	apiModules []apiModule
	logger     inter.Logger
	config     appcfg.WebConfig
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
		logger: deps.Logger,
		config: deps.Config,
	}
	ws.apiModules = buildAPIModules(deps)
	if len(ws.apiModules) == 0 {
		return nil, errors.New("web api modules are required")
	}
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
