package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

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
	db, err := DataStore.NewDataStoreSql("./data.db")
	if err != nil {
		log.Fatal(err)
	}
	im := IdentityManager.NewIdentityManager(db)
	dm := DeviceManager.NewDeviceManager(db, im)

	// Inject HTML directory path
	htmlDir := "html"
	web := Web.NewWebServer(db, dm, htmlDir)

	go web.Start()
	select {
	case <-ctx.Done():
		return
	}
}
