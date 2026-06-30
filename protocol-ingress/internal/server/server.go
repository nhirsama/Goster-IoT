package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
)

type Server struct {
	cfg       config.ServerConfig
	logger    *slog.Logger
	server    *http.Server
	startedAt time.Time
	ready     atomic.Bool
}

func New(cfg config.ServerConfig, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}
	s := &Server{cfg: cfg, logger: logger, startedAt: time.Now().UTC()}
	s.ready.Store(true)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/readyz", s.readyz)
	mux.HandleFunc("/metrics", s.metrics)
	s.server = &http.Server{Addr: cfg.HTTPAddr, Handler: mux}
	return s
}

func (s *Server) Handler() http.Handler {
	return s.server.Handler
}

func (s *Server) SetReady(ready bool) {
	s.ready.Store(ready)
}

func (s *Server) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("protocol-ingress 管理 HTTP 已启动", "addr", s.cfg.HTTPAddr)
		errCh <- s.server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		s.SetReady(false)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		s.SetReady(false)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "protocol-ingress"})
}

func (s *Server) readyz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !s.ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "not_ready", "service": "protocol-ingress"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready", "service": "protocol-ingress"})
}

func (s *Server) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	uptime := int64(time.Since(s.startedAt).Seconds())
	_, _ = w.Write([]byte("# HELP protocol_ingress_up 服务是否存活\n"))
	_, _ = w.Write([]byte("# TYPE protocol_ingress_up gauge\n"))
	_, _ = w.Write([]byte("protocol_ingress_up 1\n"))
	_, _ = w.Write([]byte("# HELP protocol_ingress_ready 服务是否就绪\n"))
	_, _ = w.Write([]byte("# TYPE protocol_ingress_ready gauge\n"))
	ready := "0"
	if s.ready.Load() {
		ready = "1"
	}
	_, _ = w.Write([]byte("protocol_ingress_ready " + ready + "\n"))
	_, _ = w.Write([]byte("# HELP protocol_ingress_uptime_seconds 服务启动后的秒数\n"))
	_, _ = w.Write([]byte("# TYPE protocol_ingress_uptime_seconds counter\n"))
	_, _ = w.Write([]byte("protocol_ingress_uptime_seconds " + strconv.FormatInt(uptime, 10) + "\n"))
}
