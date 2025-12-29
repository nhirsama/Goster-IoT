package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nhirsama/Goster-IoT/src/Api"
	"github.com/nhirsama/Goster-IoT/src/DataStore"
	"github.com/nhirsama/Goster-IoT/src/DeviceManager"
	"github.com/nhirsama/Goster-IoT/src/IdentityManager"
	"github.com/nhirsama/Goster-IoT/src/Web"
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
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data.db"
	}
	db, err := DataStore.NewDataStoreSql(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	im := IdentityManager.NewIdentityManager(db)
	dm := DeviceManager.NewDeviceManager(db, im)

	api := Api.NewApi(db, dm, im)

	htmlDir := os.Getenv("HTML_DIR")
	if htmlDir == "" {
		htmlDir = "html"
	}
	web := Web.NewWebServer(db, dm, im, api, htmlDir)
	go web.Start()
	go api.Start()
	select {
	case <-ctx.Done():
		return
	}
}
