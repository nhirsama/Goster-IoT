package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nhirsama/Goster-IoT/src/api"
	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/device_manager"
	"github.com/nhirsama/Goster-IoT/src/inter"
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
	rootLogger := initRootLogger()
	rootLogger.Info("日志系统已初始化")

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data.db"
	}
	db, err := datastore.NewDataStoreSql(dbPath)
	if err != nil {
		rootLogger.Error("数据存储初始化失败", inter.Err(err))
		panic(err)
	}

	// Initialize Authboss (Encapsulated in web package)
	ab, err := web.SetupAuthboss(db)
	if err != nil {
		rootLogger.Error("Authboss 初始化失败", inter.Err(err))
		panic(err)
	}
	authService, err := web.NewAuthService(ab)
	if err != nil {
		rootLogger.Error("认证服务初始化失败", inter.Err(err))
		panic(err)
	}

	dm := device_manager.NewDeviceManager(db)

	apiLogger := rootLogger.With(inter.String("module", "api"))
	webLogger := rootLogger.With(inter.String("module", "web"))
	api := api.NewApi(db, dm, apiLogger)

	webServer, err := web.NewWebServer(web.WebServerDeps{
		DataStore:     db,
		DeviceManager: dm,
		API:           api,
		Auth:          authService,
		Captcha:       web.NewTurnstileService(),
		Logger:        webLogger,
	})
	if err != nil {
		rootLogger.Error("Web 服务初始化失败", inter.Err(err))
		panic(err)
	}
	go webServer.Start()
	go api.Start()
	select {
	case <-ctx.Done():
		return
	}
}

func initRootLogger() inter.Logger {
	cfg := logger.ConfigFromEnv()
	root := logger.New(cfg).With(
		inter.String("module", "bootstrap"),
		inter.String("component", "cli"),
	)
	logger.SetDefault(root)
	return root
}
