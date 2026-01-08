package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nhirsama/Goster-IoT/src/api"
	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/device_manager"
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
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data.db"
	}
	db, err := datastore.NewDataStoreSql(dbPath)
	if err != nil {
		log.Fatal(err)
	}

	htmlDir := os.Getenv("HTML_DIR")
	if htmlDir == "" {
		htmlDir = "html"
	}

	// Initialize Authboss (Encapsulated in web package)
	ab, err := web.SetupAuthboss(db, htmlDir)
	if err != nil {
		log.Fatalf("Failed to setup Authboss: %v", err)
	}

	dm := device_manager.NewDeviceManager(db)

	api := api.NewApi(db, dm)

	web := web.NewWebServer(db, dm, api, htmlDir, ab)
	go web.Start()
	go api.Start()
	select {
	case <-ctx.Done():
		return
	}
}
