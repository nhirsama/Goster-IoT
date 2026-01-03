package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aarondl/authboss/v3"
	_ "github.com/aarondl/authboss/v3/auth"
	"github.com/aarondl/authboss/v3/defaults"
	_ "github.com/aarondl/authboss/v3/logout"
	_ "github.com/aarondl/authboss/v3/register"
	"github.com/gorilla/sessions"
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

	// Initialize Authboss
	ab := authboss.New()
	storer, ok := db.(authboss.ServerStorer)
	if !ok {
		log.Fatal("DataStore does not implement Authboss ServerStorer")
	}
	ab.Config.Storage.Server = storer

	// Session Store (MaxAge = 0, deleted on browser close)
	sessionStore := sessions.NewCookieStore([]byte("super-secret-key-change-me"))
	sessionStore.Options.MaxAge = 0
	sessionStore.Options.HttpOnly = true
	ab.Config.Storage.SessionState = Web.NewSessionStorer("goster_session", sessionStore)

	// Cookie Store (Remember Me, MaxAge = 30 days)
	cookieStore := sessions.NewCookieStore([]byte("super-secret-key-change-me"))
	cookieStore.Options.MaxAge = 86400 * 30
	cookieStore.Options.HttpOnly = true
	ab.Config.Storage.CookieState = Web.NewSessionStorer("goster_remember", cookieStore)

	ab.Config.Paths.Mount = "/auth"
	ab.Config.Paths.RootURL = "http://localhost:8080"

	// Basic defaults (No confirm, No lock)
	defaults.SetCore(&ab.Config, false, false)

	if err := ab.Init(); err != nil {
		log.Fatal(err)
	}

	im := IdentityManager.NewIdentityManager(db)
	dm := DeviceManager.NewDeviceManager(db, im)

	api := Api.NewApi(db, dm, im)

	htmlDir := os.Getenv("HTML_DIR")
	if htmlDir == "" {
		htmlDir = "html"
	}
	web := Web.NewWebServer(db, dm, im, api, htmlDir, ab)
	go web.Start()
	go api.Start()
	select {
	case <-ctx.Done():
		return
	}
}
