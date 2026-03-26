package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	svc := &TurnstileService{
		Enabled:   true,
		SecretKey: "secret",
		timeout:   time.Second,
		client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}
			values, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("ParseQuery failed: %v", err)
			}
			if got := values.Get("response"); got != "token-ok" {
				t.Fatalf("unexpected response token: %s", got)
			}
			if got := values.Get("remoteip"); got != "1.2.3.4" {
				t.Fatalf("unexpected remote ip: %s", got)
			}
			return httpResponse(http.StatusOK, `{"success":true}`), nil
		})},
	}
	if !svc.VerifyToken("token-ok", "1.2.3.4") {
		t.Fatal("expected successful verification")
	}
}

func TestTurnstileVerifyTokenRejectsHTTPFailureAndInvalidJSON(t *testing.T) {
	t.Run("status_non_200", func(t *testing.T) {
		svc := &TurnstileService{
			Enabled: true,
			timeout: time.Second,
			client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return httpResponse(http.StatusBadGateway, `nope`), nil
			})},
		}
		if svc.VerifyToken("token", "1.2.3.4") {
			t.Fatal("expected non-200 response to fail")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		svc := &TurnstileService{
			Enabled: true,
			timeout: time.Second,
			client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return httpResponse(http.StatusOK, `not-json`), nil
			})},
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
