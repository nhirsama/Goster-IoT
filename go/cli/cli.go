package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/device_manager"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/iot_gateway"
	"github.com/nhirsama/Goster-IoT/src/logger"
	"github.com/nhirsama/Goster-IoT/src/web"
)

func Run() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer func() {
		stop()
		fmt.Println("系统正常关闭")
	}()
	go start(ctx)
	<-ctx.Done()
}

func start(ctx context.Context) {
	appCfg, err := config.Load()
	if err != nil {
		panic(fmt.Errorf("配置加载失败: %w", err))
	}

	rootLogger := initRootLogger(appCfg.Logger)
	rootLogger.Info("日志系统已初始化")

	db, err := datastore.NewDataStoreSql(appCfg.DB.Path)
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

	dm := device_manager.NewDeviceManagerWithConfig(db, appCfg.DeviceManager)
	telemetryIngest := device_manager.NewTelemetryIngestService(db)
	downlinkQueue := device_manager.NewDeviceCommandQueue(appCfg.DeviceManager.QueueCapacity)
	downlinkCommands := device_manager.NewDownlinkCommandService(db, downlinkQueue)

	gatewayLogger := rootLogger.With(inter.String("module", "iot_gateway"))
	webLogger := rootLogger.With(inter.String("module", "web"))
	gateway := iot_gateway.NewGatewayFromCoreWithConfig(dm, dm, telemetryIngest, downlinkCommands, gatewayLogger, appCfg.API)

	webServer, err := web.NewWebServer(web.WebServerDeps{
		DataStore:        db,
		DeviceRegistry:   dm,
		DevicePresence:   dm,
		DownlinkCommands: downlinkCommands,
		Auth:             authService,
		Captcha:          web.NewTurnstileServiceWithConfig(appCfg.Captcha),
		Logger:           webLogger,
		Config:           appCfg.Web,
	})
	if err != nil {
		rootLogger.Error("Web 服务初始化失败", inter.Err(err))
		panic(err)
	}
	go webServer.Start()
	go gateway.Start()
	select {
	case <-ctx.Done():
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
