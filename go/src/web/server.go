package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

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

// Start 启动标准 HTTP 服务器，并在 ctx 取消后优雅关闭。
func (ws *webServer) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	addr := appcfg.NormalizeWebConfig(ws.config).HTTPAddr

	mux := http.NewServeMux()
	ws.registerRoutes(mux)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		ws.log().Error("Web 服务监听失败", inter.Err(err))
		return err
	}
	defer listener.Close()

	server := &http.Server{
		Handler: mux,
	}

	stopShutdown := make(chan struct{})
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdownCtx)
		case <-stopShutdown:
		}
	}()
	defer close(stopShutdown)

	ws.log().Info("Web 服务已启动", inter.String("addr", listener.Addr().String()))
	if err := server.Serve(listener); err != nil {
		if errors.Is(err, http.ErrServerClosed) && ctx.Err() != nil {
			<-shutdownDone
			return nil
		}
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
