package web

import (
	"context"
	"strings"
	"testing"
)

func TestStaticViewRendererRenderHTML(t *testing.T) {
	renderer := NewStaticViewRenderer()
	if renderer == nil {
		t.Fatal("expected renderer")
	}
	if err := renderer.Load("login", "register"); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	body, contentType, err := renderer.Render(context.Background(), "login", nil)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if contentType != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", contentType)
	}
	if !strings.Contains(string(body), "login is not supported") {
		t.Fatalf("unexpected body: %s", string(body))
	}
}
