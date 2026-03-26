package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/core"
	identitycore "github.com/nhirsama/Goster-IoT/src/identity"
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

	if err := RunWithArgs(ctx, os.Args[1:]); err != nil {
		panic(err)
	}
}

// RunWithArgs 解析并执行命令行参数。
// 当前支持:
// 1. 默认/serve: 启动整套后端服务
// 2. db init: 显式初始化数据库结构
// 3. db migrate: 当前与 init 复用同一套过渡迁移逻辑
func RunWithArgs(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return serve(ctx)
	}

	switch args[0] {
	case "serve":
		if len(args) != 1 {
			return fmt.Errorf("serve 不接受额外参数")
		}
		return serve(ctx)
	case "db":
		if len(args) < 2 {
			return fmt.Errorf("db 子命令缺失，当前支持: init, migrate")
		}
		return runDBCommand(args[1])
	default:
		return fmt.Errorf("未知命令: %s", args[0])
	}
}

// StartWithContext 以给定上下文启动整套 Go 后端服务。
// 该入口主要供集成测试和未来的外部进程托管场景复用。
func StartWithContext(ctx context.Context) {
	if err := serve(ctx); err != nil {
		panic(err)
	}
}

func start(ctx context.Context) {
	if err := serve(ctx); err != nil {
		panic(err)
	}
}

func serve(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	appCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("配置加载失败: %w", err)
	}

	rootLogger := initRootLogger(appCfg.Logger)
	rootLogger.Info("日志系统已初始化")

	dbCfg := appCfg.DB
	dbCfg.SchemaMode = "managed"

	authStore, err := persistence.OpenAuthStore(dbCfg)
	if err != nil {
		rootLogger.Error("认证存储初始化失败", inter.Err(err))
		return err
	}
	defer persistence.CloseIfPossible(authStore)

	runtimeStore, err := persistence.OpenRuntimeStore(dbCfg)
	if err != nil {
		rootLogger.Error("业务存储初始化失败", inter.Err(err))
		return err
	}
	defer persistence.CloseIfPossible(runtimeStore)

	// Initialize Authboss (Encapsulated in web package)
	ab, err := identitycore.SetupAuthbossWithConfig(authStore, appCfg.Auth)
	if err != nil {
		rootLogger.Error("Authboss 初始化失败", inter.Err(err))
		return err
	}
	authService, err := web.NewAuthService(ab)
	if err != nil {
		rootLogger.Error("认证服务初始化失败", inter.Err(err))
		return err
	}

	services := core.NewServicesWithConfig(runtimeStore, appCfg.DeviceManager)

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
		DataStore:        runtimeStore,
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
		return err
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
		return nil
	case err := <-errCh:
		if err != nil {
			rootLogger.Error("后端服务异常退出", inter.Err(err))
			cancel()
			return err
		}
		cancel()
		return nil
	}
}

func runDBCommand(action string) error {
	appCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("配置加载失败: %w", err)
	}

	rootLogger := initRootLogger(appCfg.Logger)
	switch action {
	case "init", "migrate":
		if err := persistence.EnsureSchema(appCfg.DB); err != nil {
			rootLogger.Error("数据库结构初始化失败", inter.Err(err))
			return err
		}
		rootLogger.Info("数据库结构已初始化", inter.String("action", action), inter.String("driver", appCfg.DB.Driver))
		return nil
	default:
		return errors.New("未知 db 子命令，仅支持 init / migrate")
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
