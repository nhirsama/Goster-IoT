package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nhirsama/Goster-IoT/src/logger"
)

func TestAPIMiddlewareOptionsRequest(t *testing.T) {
	ws := &webServer{}
	handlerCalled := false
	h := ws.apiMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/devices", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for preflight, got %d", rec.Code)
	}
	if handlerCalled {
		t.Fatalf("inner handler should not be called for preflight requests")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("unexpected allow-origin header: %q", got)
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Fatalf("X-Request-Id should be present on preflight response")
	}
}

func TestAPIMiddlewareRejectsDisallowedOrigin(t *testing.T) {
	ws := &webServer{}
	h := ws.apiMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("inner handler should not be called for disallowed origin")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("X-Forwarded-Host", "evil.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disallowed origin, got %d", rec.Code)
	}

	var body apiEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body.Code != 40302 {
		t.Fatalf("unexpected business code: %d", body.Code)
	}
	if body.RequestID == "" {
		t.Fatalf("request_id should be present in error envelope")
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Fatalf("X-Request-Id header should be present")
	}
}

func TestAPIMiddlewareAllowsSameOrigin(t *testing.T) {
	ws := &webServer{}
	h := ws.apiMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	req.Host = "internal.example.com:8080"
	req.Header.Set("Origin", "http://internal.example.com:8080")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for same-origin request, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://internal.example.com:8080" {
		t.Fatalf("unexpected allow-origin header: %q", got)
	}
}

func TestAPINoContentAddsRequestIDHeader(t *testing.T) {
	ws := &webServer{}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/a", nil)
	req.Header.Set("X-Request-Id", "req_test_123")
	rec := httptest.NewRecorder()

	ws.apiNoContent(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("X-Request-Id"); got != "req_test_123" {
		t.Fatalf("unexpected X-Request-Id header: %q", got)
	}
}

func TestAPIMiddlewareInjectsContextLogger(t *testing.T) {
	ws := &webServer{}
	h := ws.apiMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := logger.FromContext(r.Context())
		if l == nil {
			t.Fatal("context logger should not be nil")
		}
		l.Info("test")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
