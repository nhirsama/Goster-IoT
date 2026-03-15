package web

import (
	"os"
	"testing"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
)

func resolveCookieSecureForTest() bool {
	return appcfg.ResolveCookieSecure(
		os.Getenv("AUTH_COOKIE_SECURE"),
		os.Getenv("APP_ENV"),
		os.Getenv("AUTHBOSS_ROOT_URL"),
	)
}

func TestResolveCookieSecurePriority(t *testing.T) {
	t.Setenv("AUTH_COOKIE_SECURE", "true")
	t.Setenv("APP_ENV", "dev")
	t.Setenv("AUTHBOSS_ROOT_URL", "http://localhost:8080")

	if !resolveCookieSecureForTest() {
		t.Fatalf("AUTH_COOKIE_SECURE=true should force secure cookie")
	}
}

func TestResolveCookieSecureByEnvironment(t *testing.T) {
	t.Setenv("AUTH_COOKIE_SECURE", "")
	t.Setenv("APP_ENV", "production")
	t.Setenv("AUTHBOSS_ROOT_URL", "http://localhost:8080")

	if !resolveCookieSecureForTest() {
		t.Fatalf("production environment should enable secure cookie")
	}
}

func TestResolveCookieSecureByRootURL(t *testing.T) {
	t.Setenv("AUTH_COOKIE_SECURE", "")
	t.Setenv("APP_ENV", "dev")
	t.Setenv("AUTHBOSS_ROOT_URL", "https://example.com")

	if !resolveCookieSecureForTest() {
		t.Fatalf("https root url should enable secure cookie")
	}
}

func TestResolveCookieSecureDefaultFalse(t *testing.T) {
	t.Setenv("AUTH_COOKIE_SECURE", "")
	t.Setenv("APP_ENV", "dev")
	t.Setenv("AUTHBOSS_ROOT_URL", "http://localhost:8080")

	if resolveCookieSecureForTest() {
		t.Fatalf("non-production http environment should disable secure cookie by default")
	}
}
