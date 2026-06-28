package identity

import (
	"context"
	"strings"
	"testing"
)

func TestStaticViewRenderer(t *testing.T) {
	renderer := NewStaticViewRenderer()
	if err := renderer.Load("login"); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	body, contentType, err := renderer.Render(context.Background(), "login", nil)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if contentType != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", contentType)
	}
	if !strings.Contains(string(body), "login is not supported on this server") {
		t.Fatalf("unexpected render body: %s", string(body))
	}
}
