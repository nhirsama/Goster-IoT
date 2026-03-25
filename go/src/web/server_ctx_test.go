package web

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/core"
	"github.com/nhirsama/Goster-IoT/src/persistence"
)

func TestWebServerStartStopsOnContextCancel(t *testing.T) {
	listener := newWebListener(t)
	addr := listener.Addr().String()
	dbPath := filepath.Join(t.TempDir(), "web_ctx.db")

	ds, err := persistence.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to open datastore: %v", err)
	}
	services := core.NewServices(ds)

	ab, err := SetupAuthboss(ds)
	if err != nil {
		t.Fatalf("failed to setup authboss: %v", err)
	}
	authService, err := NewAuthService(ab)
	if err != nil {
		t.Fatalf("failed to setup auth service: %v", err)
	}

	ws, err := NewWebServer(WebServerDeps{
		DataStore:        ds,
		DeviceRegistry:   services.DeviceRegistry,
		DevicePresence:   services.DevicePresence,
		DownlinkCommands: services.DownlinkCommands,
		Auth:             authService,
		Captcha:          &TurnstileService{Enabled: false},
		Config: appcfg.WebConfig{
			HTTPAddr: addr,
		},
	})
	if err != nil {
		t.Fatalf("failed to create web server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- ws.Serve(ctx, listener)
	}()

	waitForWebHTTPServer(t, "http://"+addr+"/api/v1/auth/captcha/config")
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("web server should stop cleanly on context cancel: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("web server did not stop after context cancel")
	}
}

func newWebListener(t *testing.T) net.Listener {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve tcp address: %v", err)
	}
	return l
}

func waitForWebHTTPServer(t *testing.T, url string) {
	t.Helper()

	client := &http.Client{Timeout: 300 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("http server did not start at %s", url)
}
