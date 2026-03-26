package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

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

func TestTurnstileVerifyRejectsMissingFormToken(t *testing.T) {
	svc := &TurnstileService{Enabled: true}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	if svc.Verify(req) {
		t.Fatal("expected missing form token to be rejected")
	}
}

func TestTurnstileVerifyTokenAcceptsSuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		if got := r.FormValue("response"); got != "token-ok" {
			t.Fatalf("unexpected response token: %s", got)
		}
		if got := r.FormValue("remoteip"); got != "1.2.3.4" {
			t.Fatalf("unexpected remote ip: %s", got)
		}
		_, _ = io.WriteString(w, `{"success":true}`)
	}))
	defer server.Close()

	svc := &TurnstileService{
		Enabled:   true,
		SecretKey: "secret",
		timeout:   time.Second,
		client:    &http.Client{Transport: rewriteTransport(t, server.URL)},
	}
	if !svc.VerifyToken("token-ok", "1.2.3.4") {
		t.Fatal("expected successful verification")
	}
}

func TestTurnstileVerifyTokenRejectsHTTPFailureAndInvalidJSON(t *testing.T) {
	t.Run("status_non_200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusBadGateway)
		}))
		defer server.Close()

		svc := &TurnstileService{
			Enabled: true,
			timeout: time.Second,
			client:  &http.Client{Transport: rewriteTransport(t, server.URL)},
		}
		if svc.VerifyToken("token", "1.2.3.4") {
			t.Fatal("expected non-200 response to fail")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `not-json`)
		}))
		defer server.Close()

		svc := &TurnstileService{
			Enabled: true,
			timeout: time.Second,
			client:  &http.Client{Transport: rewriteTransport(t, server.URL)},
		}
		if svc.VerifyToken("token", "1.2.3.4") {
			t.Fatal("expected invalid json to fail")
		}
	})
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

type rewriteRoundTripper struct {
	base   http.RoundTripper
	target *url.URL
}

func (rt rewriteRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = rt.target.Scheme
	cloned.URL.Host = rt.target.Host
	cloned.Host = rt.target.Host
	return rt.base.RoundTrip(cloned)
}

func rewriteTransport(t *testing.T, rawURL string) http.RoundTripper {
	t.Helper()

	target, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse target url failed: %v", err)
	}
	return rewriteRoundTripper{
		base:   http.DefaultTransport,
		target: target,
	}
}
