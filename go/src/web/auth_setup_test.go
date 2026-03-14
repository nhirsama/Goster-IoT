package web

import "testing"

func TestResolveCookieSecurePriority(t *testing.T) {
	t.Setenv("AUTH_COOKIE_SECURE", "true")
	t.Setenv("APP_ENV", "dev")
	t.Setenv("AUTHBOSS_ROOT_URL", "http://localhost:8080")

	if !resolveCookieSecure() {
		t.Fatalf("AUTH_COOKIE_SECURE=true should force secure cookie")
	}
}

func TestResolveCookieSecureByEnvironment(t *testing.T) {
	t.Setenv("AUTH_COOKIE_SECURE", "")
	t.Setenv("APP_ENV", "production")
	t.Setenv("AUTHBOSS_ROOT_URL", "http://localhost:8080")

	if !resolveCookieSecure() {
		t.Fatalf("production environment should enable secure cookie")
	}
}

func TestResolveCookieSecureByRootURL(t *testing.T) {
	t.Setenv("AUTH_COOKIE_SECURE", "")
	t.Setenv("APP_ENV", "dev")
	t.Setenv("AUTHBOSS_ROOT_URL", "https://example.com")

	if !resolveCookieSecure() {
		t.Fatalf("https root url should enable secure cookie")
	}
}

func TestResolveCookieSecureDefaultFalse(t *testing.T) {
	t.Setenv("AUTH_COOKIE_SECURE", "")
	t.Setenv("APP_ENV", "dev")
	t.Setenv("AUTHBOSS_ROOT_URL", "http://localhost:8080")

	if resolveCookieSecure() {
		t.Fatalf("non-production http environment should disable secure cookie by default")
	}
}
