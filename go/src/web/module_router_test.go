package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1/ingressv1connect"
	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/core"
	identitycore "github.com/nhirsama/Goster-IoT/src/identity"
	"github.com/nhirsama/Goster-IoT/src/inter"
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

func TestIngressRoutesRequireBearerTokenWhenConfigured(t *testing.T) {
	deps := newTestWebDeps(t)
	store := deps.DataStore.(inter.CoreStore)
	deps.IngressStore = store
	deps.TelemetryIngest = core.NewServices(store).TelemetryIngest
	deps.IngressToken = "shared-secret"

	ws, err := newWebServer(deps)
	if err != nil {
		t.Fatalf("newWebServer failed: %v", err)
	}

	mux := http.NewServeMux()
	ws.registerRoutes(mux)
	path := ingressv1connect.ProtocolIngressCoreServiceAuthenticateDeviceProcedure

	for _, tc := range []struct {
		name   string
		header string
	}{
		{name: "missing"},
		{name: "wrong", header: "Bearer wrong-secret"},
		{name: "bad_scheme", header: "Basic shared-secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected unauthorized, got %d body=%s", rec.Code, rec.Body.String())
			}
			if rec.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("expected WWW-Authenticate header")
			}
		})
	}

	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.Header.Set("Authorization", "Bearer shared-secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("valid token should pass auth middleware, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestIngressRoutesAllowRequestsWhenTokenUnset(t *testing.T) {
	deps := newTestWebDeps(t)
	store := deps.DataStore.(inter.CoreStore)
	deps.IngressStore = store
	deps.TelemetryIngest = core.NewServices(store).TelemetryIngest

	ws, err := newWebServer(deps)
	if err != nil {
		t.Fatalf("newWebServer failed: %v", err)
	}

	mux := http.NewServeMux()
	ws.registerRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, ingressv1connect.ProtocolIngressCoreServiceAuthenticateDeviceProcedure, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("empty ingress token should not enable middleware, got %d", rec.Code)
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
	ab, err := identitycore.SetupAuthbossWithConfig(ds, appcfg.DefaultAuthConfig())
	if err != nil {
		t.Fatalf("SetupAuthbossWithConfig failed: %v", err)
	}
	authService, err := identitycore.NewAuthbossService(ab)
	if err != nil {
		t.Fatalf("NewAuthbossService failed: %v", err)
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
