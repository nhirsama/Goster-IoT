package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/core"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/iot_gateway"
	"github.com/nhirsama/Goster-IoT/src/logger"
	"github.com/nhirsama/Goster-IoT/src/persistence"
	"github.com/nhirsama/Goster-IoT/src/web"
)

func Run() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer func() {
		stop()
		fmt.Println("系统正常关闭")
	}()
	go StartWithContext(ctx)
	<-ctx.Done()
}

// StartWithContext 以给定上下文启动整套 Go 后端服务。
// 该入口主要供集成测试和未来的外部进程托管场景复用。
func StartWithContext(ctx context.Context) {
	start(ctx)
}

func start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	appCfg, err := config.Load()
	if err != nil {
		panic(fmt.Errorf("配置加载失败: %w", err))
	}

	rootLogger := initRootLogger(appCfg.Logger)
	rootLogger.Info("日志系统已初始化")

	db, err := persistence.OpenStore(appCfg.DB)
	if err != nil {
		rootLogger.Error("数据存储初始化失败", inter.Err(err))
		panic(err)
	}

	// Initialize Authboss (Encapsulated in web package)
	ab, err := web.SetupAuthbossWithConfig(db, appCfg.Auth)
	if err != nil {
		rootLogger.Error("Authboss 初始化失败", inter.Err(err))
		panic(err)
	}
	authService, err := web.NewAuthService(ab)
	if err != nil {
		rootLogger.Error("认证服务初始化失败", inter.Err(err))
		panic(err)
	}

	services := core.NewServicesWithConfig(db, appCfg.DeviceManager)

	gatewayLogger := rootLogger.With(inter.String("module", "iot_gateway"))
	webLogger := rootLogger.With(inter.String("module", "web"))
	gateway := iot_gateway.NewGatewayFromCoreWithConfig(
		services.DeviceRegistry,
		services.DevicePresence,
		services.TelemetryIngest,
		services.DownlinkCommands,
		gatewayLogger,
		appCfg.API,
	)

	webServer, err := web.NewWebServer(web.WebServerDeps{
		DataStore:        db,
		DeviceRegistry:   services.DeviceRegistry,
		DevicePresence:   services.DevicePresence,
		DownlinkCommands: services.DownlinkCommands,
		Auth:             authService,
		Captcha:          web.NewTurnstileServiceWithConfig(appCfg.Captcha),
		Logger:           webLogger,
		Config:           appCfg.Web,
	})
	if err != nil {
		rootLogger.Error("Web 服务初始化失败", inter.Err(err))
		panic(err)
	}

	errCh := make(chan error, 2)
	go func() {
		errCh <- webServer.Start(ctx)
	}()
	go func() {
		errCh <- gateway.Start(ctx)
	}()

	select {
	case <-ctx.Done():
		return
	case err := <-errCh:
		if err != nil {
			rootLogger.Error("后端服务异常退出", inter.Err(err))
		}
		cancel()
		return
	}
}

func initRootLogger(cfg logger.Config) inter.Logger {
	root := logger.New(cfg).With(
		inter.String("module", "bootstrap"),
		inter.String("component", "cli"),
	)
	logger.SetDefault(root)
	return root
}
