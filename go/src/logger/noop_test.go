package logger

import (
	"context"
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestNoopLogger(t *testing.T) {
	l := NewNoop()
	l.Debug("debug", inter.String("k", "v"))
	l.Info("info")
	l.Warn("warn")
	l.Error("error")
	l.DebugContext(context.Background(), "debug")
	l.InfoContext(context.Background(), "info")
	l.WarnContext(context.Background(), "warn")
	l.ErrorContext(context.Background(), "error")

	if l.With(inter.String("a", "b")) == nil {
		t.Fatal("With returned nil")
	}
	if l.WithGroup("g") == nil {
		t.Fatal("WithGroup returned nil")
	}
}
