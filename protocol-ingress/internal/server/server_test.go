package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
)

func TestHealthReadyAndMetricsHandlers(t *testing.T) {
	s := New(config.ServerConfig{HTTPAddr: "127.0.0.1:0", ShutdownTimeout: time.Second}, slog.Default())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz status=%d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode healthz: %v", err)
	}
	if body["status"] != "ok" || body["service"] != "protocol-ingress" {
		t.Fatalf("unexpected healthz body: %v", body)
	}

	s.SetReady(false)
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "protocol_ingress_up 1") {
		t.Fatalf("unexpected metrics: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStartStopsOnContextCancel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("tcp listener unavailable: %v", err)
	}
	_ = listener.Close()

	s := New(config.ServerConfig{HTTPAddr: listener.Addr().String(), ShutdownTimeout: time.Second}, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.Start(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + listener.Addr().String() + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Fatalf("unexpected Start error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop")
	}
}
