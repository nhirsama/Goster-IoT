package web

import (
	"net/http/httptest"
	"testing"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
)

func TestNewTurnstileServiceWithConfigEnablesProvider(t *testing.T) {
	svc := NewTurnstileServiceWithConfig(appcfg.CaptchaConfig{
		Provider: "turnstile",
		SiteKey:  "site",
	})
	if !svc.IsEnabled() {
		t.Fatal("expected turnstile service to be enabled")
	}
	if svc.PublicSiteKey() != "site" {
		t.Fatalf("unexpected site key: %s", svc.PublicSiteKey())
	}
}

func TestTurnstileVerifyTokenDisabledReturnsTrue(t *testing.T) {
	svc := &TurnstileService{Enabled: false}
	if !svc.VerifyToken("", "") {
		t.Fatal("disabled service should bypass verification")
	}
}

func TestTurnstileVerifyTokenRejectsEmptyTokenWhenEnabled(t *testing.T) {
	svc := &TurnstileService{Enabled: true}
	if svc.VerifyToken("", "") {
		t.Fatal("enabled service should reject empty token")
	}
}

func TestClientIPFromRequestPrefersForwardedHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
	req.Header.Set("X-Real-IP", "3.3.3.3")
	req.RemoteAddr = "4.4.4.4:1234"

	if got := clientIPFromRequest(req); got != "1.1.1.1" {
		t.Fatalf("unexpected client ip: %s", got)
	}
}
