package logger

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestSlogLoggerJSONAndRedact(t *testing.T) {
	var buf bytes.Buffer
	l := newWithWriter(Config{
		Level:   "debug",
		Format:  "json",
		Service: "svc",
		Env:     "test",
	}, &buf)

	l.With(inter.String("module", "api")).
		WithGroup("auth").
		InfoContext(context.Background(), "login failed",
			inter.String("token", "abc123"),
			inter.String("username", "alice"))

	out := buf.String()
	mustContain(t, out, `"msg":"login failed"`)
	mustContain(t, out, `"service":"svc"`)
	mustContain(t, out, `"env":"test"`)
	mustContain(t, out, `"module":"api"`)
	mustContain(t, out, `"token":"***"`)
	mustContain(t, out, `"username":"alice"`)
}

func TestSlogLoggerLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	l := newWithWriter(Config{
		Level:   "error",
		Format:  "json",
		Service: "svc",
		Env:     "test",
	}, &buf)

	l.Info("hidden")
	l.Error("visible")

	out := buf.String()
	if strings.Contains(out, `"msg":"hidden"`) {
		t.Fatalf("info log should be filtered: %s", out)
	}
	mustContain(t, out, `"msg":"visible"`)
}

func mustContain(t *testing.T, s string, want string) {
	t.Helper()
	if !strings.Contains(s, want) {
		t.Fatalf("output missing %s, got: %s", want, s)
	}
}
