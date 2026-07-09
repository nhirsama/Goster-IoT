package v1_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondl/authboss/v3"
	identitycore "github.com/nhirsama/Goster-IoT/src/identity"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestAPIAccessControlHandler(t *testing.T) {
	env := newTestAPI(t)
	uuid := strings.Repeat("d", 64)
	seedDevice(t, env.dataStore, uuid, inter.Authenticated)

	if err := env.dataStore.BatchAppendMetrics(uuid, []inter.MetricPoint{
		{Timestamp: 1700000000000, Value: 0, Type: 8},
		{Timestamp: 1700000000100, Value: 1, Type: 16},
		{Timestamp: 1700000000200, Value: 1, Type: 8},
	}); err != nil {
		t.Fatalf("seed access-control metrics failed: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/access-control/"+uuid, nil)
	env.api.AccessControlHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("access control expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	data := mustJSONEnvelope(t, rec).Data.(map[string]interface{})
	if data["uuid"] != uuid {
		t.Fatalf("unexpected uuid: %+v", data)
	}
	if data["signal_a"] != float64(1) || data["signal_b"] != float64(1) || data["open"] != true || data["status_text"] != "open" {
		t.Fatalf("unexpected open access-control data: %+v", data)
	}
	if data["evaluated_at_ms"] != float64(1700000000200) {
		t.Fatalf("unexpected evaluated_at_ms: %+v", data)
	}
}

func TestAPIAccessControlHandlerUnknownWhenSignalMissing(t *testing.T) {
	env := newTestAPI(t)
	uuid := strings.Repeat("e", 64)
	seedDevice(t, env.dataStore, uuid, inter.Authenticated)

	if err := env.dataStore.BatchAppendMetrics(uuid, []inter.MetricPoint{
		{Timestamp: 1700000000000, Value: 1, Type: 8},
	}); err != nil {
		t.Fatalf("seed access-control metrics failed: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/access-control/"+uuid, nil)
	env.api.AccessControlHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("access control expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	data := mustJSONEnvelope(t, rec).Data.(map[string]interface{})
	if data["signal_a"] != float64(1) || data["signal_b"] != nil || data["open"] != nil || data["status_text"] != "unknown" {
		t.Fatalf("unexpected unknown access-control data: %+v", data)
	}
}

func TestAPIAccessControlRegisteredRouteComputesClosedState(t *testing.T) {
	env := newTestAPI(t)
	mux := newTestMux(env.api)
	username := "access_viewer"
	seedUser(t, env.dataStore, username)
	if err := env.dataStore.AddTenantUser(inter.DefaultTenantID, username, inter.TenantRoleRO); err != nil {
		t.Fatalf("failed to seed viewer tenant role: %v", err)
	}

	uuid := strings.Repeat("f", 64)
	seedDevice(t, env.dataStore, uuid, inter.Authenticated)
	if err := env.dataStore.BatchAppendMetrics(uuid, []inter.MetricPoint{
		{Timestamp: 1700000000000, Value: 1, Type: 8},
		{Timestamp: 1700000000100, Value: 1, Type: 16},
		{Timestamp: 1700000000200, Value: 0, Type: 16},
	}); err != nil {
		t.Fatalf("seed access-control metrics failed: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/access-control/"+uuid, nil)
	req.Header.Set("X-Tenant-Id", inter.DefaultTenantID)
	req = req.WithContext(context.WithValue(req.Context(), authboss.CTXKeyUser, &identitycore.AuthUser{
		Username:   username,
		Permission: int(inter.PermissionReadOnly),
	}))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("registered access-control route expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	data := mustJSONEnvelope(t, rec).Data.(map[string]interface{})
	if data["open"] != false || data["status_text"] != "closed" {
		t.Fatalf("unexpected closed access-control data: %+v", data)
	}
}
