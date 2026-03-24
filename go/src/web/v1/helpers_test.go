package v1_test

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
	apiv1 "github.com/nhirsama/Goster-IoT/src/web/v1"
)

func TestParseDeviceStatusFilter(t *testing.T) {
	status, ptr, err := apiv1.ParseDeviceStatusFilter("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "authenticated" {
		t.Fatalf("expected default authenticated, got %s", status)
	}
	if ptr == nil || *ptr != inter.Authenticated {
		t.Fatalf("expected authenticated pointer")
	}

	status, ptr, err = apiv1.ParseDeviceStatusFilter("all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "all" || ptr != nil {
		t.Fatalf("expected all with nil ptr")
	}

	if _, _, err := apiv1.ParseDeviceStatusFilter("bad-status"); err == nil {
		t.Fatalf("invalid status should return error")
	}
}

func TestDecodeAPIBody(t *testing.T) {
	var valid struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/test", bytes.NewBufferString(`{"name":"ok"}`))
	if err := apiv1.DecodeBody(req, &valid, 1<<20); err != nil {
		t.Fatalf("valid json should pass: %v", err)
	}
	if valid.Name != "ok" {
		t.Fatalf("unexpected decoded value: %s", valid.Name)
	}

	reqUnknown := httptest.NewRequest(http.MethodPost, "/api/v1/test", bytes.NewBufferString(`{"name":"ok","extra":1}`))
	if err := apiv1.DecodeBody(reqUnknown, &valid, 1<<20); err == nil {
		t.Fatalf("unknown field should fail")
	}

	reqMulti := httptest.NewRequest(http.MethodPost, "/api/v1/test", bytes.NewBufferString(`{"name":"ok"}{"name":"next"}`))
	if err := apiv1.DecodeBody(reqMulti, &valid, 1<<20); err == nil {
		t.Fatalf("multiple json docs should fail")
	}
}

func TestSameOriginChecksIgnoreForwardedHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	req.Host = "api.example.com"
	if !apiv1.IsSameOriginRequest(req, "http://api.example.com") {
		t.Fatalf("same host + http should be same-origin")
	}

	reqTLS := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	reqTLS.Host = "api.example.com"
	reqTLS.TLS = &tls.ConnectionState{}
	if !apiv1.IsSameOriginRequest(reqTLS, "https://api.example.com") {
		t.Fatalf("tls-backed https should be same-origin")
	}

	reqProxyHost := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	reqProxyHost.Host = "internal:8080"
	reqProxyHost.Header.Set("X-Forwarded-Host", "api.example.com")
	reqProxyHost.Header.Set("X-Forwarded-Proto", "https")
	if apiv1.IsSameOriginRequest(reqProxyHost, "https://api.example.com") {
		t.Fatalf("forwarded host must not be trusted for same-origin checks")
	}

	if apiv1.IsSameOriginRequest(req, "https://other.example.com") {
		t.Fatalf("different host should not be same-origin")
	}

	reqForwardedProto := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	reqForwardedProto.Host = "api.example.com"
	reqForwardedProto.Header.Set("X-Forwarded-Proto", "https")
	if apiv1.IsSameOriginRequest(reqForwardedProto, "https://api.example.com") {
		t.Fatalf("forwarded proto must not be trusted for same-origin checks")
	}
}

func TestResolveAllowedAPIOrigin(t *testing.T) {
	env := newTestAPI(t, apiTestOptions{
		config: appcfg.WebConfig{
			APICORSAllowOrigins: "https://fe.example.com,https://admin.example.com",
		},
	})

	reqSame := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	reqSame.Host = "api.internal.local:8080"
	if origin, ok := env.api.ResolveAllowedOrigin(reqSame, "http://api.internal.local:8080"); !ok || origin == "" {
		t.Fatalf("same-origin should be allowed")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	req.Host = "api.example.com"
	if _, ok := env.api.ResolveAllowedOrigin(req, "https://fe.example.com"); !ok {
		t.Fatalf("whitelisted origin should be allowed")
	}
	if _, ok := env.api.ResolveAllowedOrigin(req, "https://evil.example.com"); ok {
		t.Fatalf("non-whitelisted origin should be rejected")
	}
}

func TestAPIDevicesHandlerRejectsInvalidQueries(t *testing.T) {
	env := newTestAPI(t)
	seedDevice(t, env.dataStore, strings.Repeat("a", 64), inter.Authenticated)

	cases := []struct {
		name string
		url  string
	}{
		{
			name: "invalid_status",
			url:  "/api/v1/devices?status=bad",
		},
		{
			name: "invalid_page",
			url:  "/api/v1/devices?page=0",
		},
		{
			name: "invalid_size",
			url:  "/api/v1/devices?size=999999",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			rec := httptest.NewRecorder()

			env.api.DevicesHandler(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", rec.Code)
			}

			var envBody apiv1.Envelope
			if err := json.Unmarshal(rec.Body.Bytes(), &envBody); err != nil {
				t.Fatalf("decode response failed: %v", err)
			}
			if envBody.Code == 0 {
				t.Fatalf("expected non-zero business code for invalid query")
			}
		})
	}
}
