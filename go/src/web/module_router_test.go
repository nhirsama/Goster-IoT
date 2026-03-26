package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/nhirsama/Goster-IoT/src/core"
	"github.com/nhirsama/Goster-IoT/src/logger"
	"github.com/nhirsama/Goster-IoT/src/persistence"
)

func TestBuildAPIModulesReturnsV1Module(t *testing.T) {
	deps := newTestWebDeps(t)
	modules := buildAPIModules(deps)
	if len(modules) != 1 {
		t.Fatalf("expected 1 api module, got %d", len(modules))
	}
}

func TestRegisterRoutesExposesV1Endpoints(t *testing.T) {
	deps := newTestWebDeps(t)
	ws, err := newWebServer(deps)
	if err != nil {
		t.Fatalf("newWebServer failed: %v", err)
	}

	mux := http.NewServeMux()
	ws.registerRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/captcha/config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusNotFound {
		t.Fatal("expected captcha config route to be registered")
	}
}

func TestServeRejectsNilListener(t *testing.T) {
	ws := &webServer{}
	if err := ws.Serve(nil, nil); err == nil {
		t.Fatal("expected nil listener error")
	}
}

func newTestWebDeps(t *testing.T) WebServerDeps {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "web_test.db")
	ds, err := persistence.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() {
		_ = persistence.CloseIfPossible(ds)
	})

	services := core.NewServices(ds)
	ab, err := SetupAuthboss(ds)
	if err != nil {
		t.Fatalf("SetupAuthboss failed: %v", err)
	}
	authService, err := NewAuthService(ab)
	if err != nil {
		t.Fatalf("NewAuthService failed: %v", err)
	}

	return WebServerDeps{
		DataStore:        ds,
		DeviceRegistry:   services.DeviceRegistry,
		DevicePresence:   services.DevicePresence,
		DownlinkCommands: services.DownlinkCommands,
		Auth:             authService,
		Captcha:          &TurnstileService{Enabled: false},
		Logger:           logger.NewNoop(),
	}
}
