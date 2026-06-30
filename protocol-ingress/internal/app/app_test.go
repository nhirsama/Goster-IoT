package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
)

type stubAdapter struct {
	name    string
	err     error
	started atomic.Bool
	block   bool
}

func (s *stubAdapter) Name() string { return s.name }
func (s *stubAdapter) Start(ctx context.Context) error {
	s.started.Store(true)
	if s.err != nil {
		return s.err
	}
	if s.block {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

func TestRunReturnsAdapterErrorAndCancels(t *testing.T) {
	want := errors.New("adapter failed")
	blocking := &stubAdapter{name: "blocking", block: true}
	failing := &stubAdapter{name: "failing", err: want}
	a := New(config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)), WithServer(nil), WithAdapters(blocking, failing))
	err := a.Run(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocking.started.Load() || !failing.started.Load() {
		t.Fatalf("adapters not started")
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	adapter := &stubAdapter{name: "blocking", block: true}
	a := New(config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)), WithServer(nil), WithAdapters(adapter))
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- a.Run(ctx) }()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && !adapter.started.Load() {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop")
	}
}
