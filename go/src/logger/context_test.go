package logger

import (
	"context"
	"testing"
)

func TestContextLogger(t *testing.T) {
	old := Default()
	t.Cleanup(func() {
		SetDefault(old)
	})

	def := NewNoop()
	SetDefault(def)

	if FromContext(nil) == nil {
		t.Fatal("nil context should return default logger")
	}

	ctx := IntoContext(context.Background(), nil)
	if FromContext(ctx) == nil {
		t.Fatal("context logger should fallback to default")
	}

	custom := NewNoop()
	ctx = IntoContext(context.Background(), custom)
	if FromContext(ctx) == nil {
		t.Fatal("context logger should exist")
	}
}
