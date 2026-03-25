package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_PATH", "")
	t.Setenv("DB_DSN", "")
	t.Setenv("DB_SCHEMA_MODE", "")
	t.Setenv("WEB_HTTP_ADDR", "")
	t.Setenv("API_TCP_ADDR", "")
	t.Setenv("API_CORS_ALLOW_ORIGINS", "")
	t.Setenv("WEB_API_MAX_BODY_BYTES", "")
	t.Setenv("WEB_DEVICE_LIST_DEFAULT_PAGE_SIZE", "")
	t.Setenv("WEB_DEVICE_LIST_MAX_PAGE_SIZE", "")
	t.Setenv("WEB_METRICS_MIN_VALID_TIMESTAMP_MS", "")
	t.Setenv("WEB_METRICS_DEFAULT_RANGE_LABEL", "")
	t.Setenv("WEB_LOGIN_MAX_FAILURES", "")
	t.Setenv("WEB_LOGIN_WINDOW", "")
	t.Setenv("WEB_LOGIN_LOCKOUT", "")
	t.Setenv("API_READ_TIMEOUT", "")
	t.Setenv("API_REGISTER_ACK_GRACE_DELAY", "")
	t.Setenv("AUTHBOSS_ROOT_URL", "")
	t.Setenv("AUTH_COOKIE_SECURE", "")
	t.Setenv("AUTH_SESSION_COOKIE_MAX_AGE_SECONDS", "")
	t.Setenv("AUTH_REMEMBER_COOKIE_MAX_AGE_SECONDS", "")
	t.Setenv("CAPTCHA_VERIFY_TIMEOUT", "")
	t.Setenv("DM_QUEUE_CAPACITY", "")
	t.Setenv("DM_HEARTBEAT_DEADLINE", "")
	t.Setenv("DM_EXTERNAL_LIST_DEFAULT_SIZE", "")
	t.Setenv("DM_EXTERNAL_LIST_MAX_SIZE", "")
	t.Setenv("DM_EXTERNAL_OBS_DEFAULT_LIMIT", "")
	t.Setenv("DM_EXTERNAL_OBS_MAX_LIMIT", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "")
	t.Setenv("LOG_ADD_SOURCE", "")
	t.Setenv("LOG_SERVICE", "")
	t.Setenv("LOG_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DB.Driver != defaultDBDriver || cfg.DB.Path != defaultDBPath || cfg.DB.DSN != "" || cfg.DB.SchemaMode != defaultSQLiteSchemaMode {
		t.Fatalf("unexpected db config: %+v", cfg.DB)
	}
	if cfg.Web.HTTPAddr != defaultWebHTTPAddr {
		t.Fatalf("unexpected web http addr: %s", cfg.Web.HTTPAddr)
	}
	if cfg.API.TCPAddr != defaultAPITCPAddr {
		t.Fatalf("unexpected api tcp addr: %s", cfg.API.TCPAddr)
	}
	if cfg.Web.APICORSAllowOrigins != defaultAPICORSAllowOrigins {
		t.Fatalf("unexpected cors allow origins: %s", cfg.Web.APICORSAllowOrigins)
	}
	if cfg.Web.MaxAPIBodyBytes != defaultMaxAPIBodyBytes {
		t.Fatalf("unexpected max api body bytes: %d", cfg.Web.MaxAPIBodyBytes)
	}
	if cfg.Web.DeviceListPage.DefaultSize != 100 || cfg.Web.DeviceListPage.MaxSize != 1000 {
		t.Fatalf("unexpected web device list page config: %+v", cfg.Web.DeviceListPage)
	}
	if cfg.Web.Metrics.MinValidTimestampMs != defaultMetricsMinValidTimestampMs || cfg.Web.Metrics.DefaultRangeLabel != defaultMetricsRangeLabel {
		t.Fatalf("unexpected web metrics config: %+v", cfg.Web.Metrics)
	}
	if cfg.Web.LoginProtection.MaxFailures != defaultLoginMaxFailures || cfg.Web.LoginProtection.Window != defaultLoginFailureWindow || cfg.Web.LoginProtection.Lockout != defaultLoginLockout {
		t.Fatalf("unexpected login protection config: %+v", cfg.Web.LoginProtection)
	}
	if cfg.API.ReadTimeout != 60*time.Second || cfg.API.RegisterAckGraceDelay != 100*time.Millisecond {
		t.Fatalf("unexpected api runtime config: %+v", cfg.API)
	}
	if cfg.Auth.RootURL != defaultAuthRootURL {
		t.Fatalf("unexpected auth root url: %s", cfg.Auth.RootURL)
	}
	if cfg.Auth.SessionCookieMaxAgeSeconds != 0 || cfg.Auth.RememberCookieMaxAgeSeconds != 86400*30 {
		t.Fatalf("unexpected auth cookie ages: %+v", cfg.Auth)
	}
	if cfg.Captcha.VerifyTimeout != 5*time.Second {
		t.Fatalf("unexpected captcha verify timeout: %v", cfg.Captcha.VerifyTimeout)
	}
	if cfg.DeviceManager.QueueCapacity != 100 || cfg.DeviceManager.HeartbeatDeadline != 60*time.Second {
		t.Fatalf("unexpected device manager base config: %+v", cfg.DeviceManager)
	}
	if cfg.DeviceManager.ExternalListPage.DefaultSize != 100 || cfg.DeviceManager.ExternalListPage.MaxSize != 1000 {
		t.Fatalf("unexpected device manager external list config: %+v", cfg.DeviceManager.ExternalListPage)
	}
	if cfg.DeviceManager.ExternalObservationLimit.Default != 1000 || cfg.DeviceManager.ExternalObservationLimit.Max != 10000 {
		t.Fatalf("unexpected device manager external obs config: %+v", cfg.DeviceManager.ExternalObservationLimit)
	}
	if cfg.Logger.Level != "info" || cfg.Logger.Format != "text" || cfg.Logger.Env != "dev" {
		t.Fatalf("unexpected logger defaults: %+v", cfg.Logger)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_PATH", "/tmp/custom.db")
	t.Setenv("DB_DSN", "")
	t.Setenv("DB_SCHEMA_MODE", "managed")
	t.Setenv("WEB_HTTP_ADDR", ":9000")
	t.Setenv("API_TCP_ADDR", ":9001")
	t.Setenv("API_CORS_ALLOW_ORIGINS", "https://fe.example.com")
	t.Setenv("WEB_API_MAX_BODY_BYTES", "2097152")
	t.Setenv("WEB_DEVICE_LIST_DEFAULT_PAGE_SIZE", "30")
	t.Setenv("WEB_DEVICE_LIST_MAX_PAGE_SIZE", "2000")
	t.Setenv("WEB_METRICS_MIN_VALID_TIMESTAMP_MS", "1700000000000")
	t.Setenv("WEB_METRICS_DEFAULT_RANGE_LABEL", "24h")
	t.Setenv("WEB_LOGIN_MAX_FAILURES", "7")
	t.Setenv("WEB_LOGIN_WINDOW", "20m")
	t.Setenv("WEB_LOGIN_LOCKOUT", "45m")
	t.Setenv("API_READ_TIMEOUT", "75s")
	t.Setenv("API_REGISTER_ACK_GRACE_DELAY", "250ms")
	t.Setenv("AUTHBOSS_ROOT_URL", "https://iot.example.com")
	t.Setenv("AUTH_COOKIE_SECURE", "false")
	t.Setenv("AUTH_SESSION_COOKIE_MAX_AGE_SECONDS", "120")
	t.Setenv("AUTH_REMEMBER_COOKIE_MAX_AGE_SECONDS", "86400")
	t.Setenv("GITHUB_CLIENT_ID", "gh_id")
	t.Setenv("GITHUB_CLIENT_SECRET", "gh_secret")
	t.Setenv("CAPTCHA_PROVIDER", "turnstile")
	t.Setenv("CF_SITE_KEY", "site_key")
	t.Setenv("CF_SECRET_KEY", "secret_key")
	t.Setenv("CAPTCHA_VERIFY_TIMEOUT", "3s")
	t.Setenv("DM_QUEUE_CAPACITY", "256")
	t.Setenv("DM_HEARTBEAT_DEADLINE", "90s")
	t.Setenv("DM_EXTERNAL_LIST_DEFAULT_SIZE", "50")
	t.Setenv("DM_EXTERNAL_LIST_MAX_SIZE", "500")
	t.Setenv("DM_EXTERNAL_OBS_DEFAULT_LIMIT", "2000")
	t.Setenv("DM_EXTERNAL_OBS_MAX_LIMIT", "30000")
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_ADD_SOURCE", "true")
	t.Setenv("LOG_SERVICE", "iot-backend")
	t.Setenv("LOG_ENV", "test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DB.Driver != "sqlite" || cfg.DB.Path != "/tmp/custom.db" || cfg.DB.DSN != "" || cfg.DB.SchemaMode != "managed" {
		t.Fatalf("unexpected db config: %+v", cfg.DB)
	}
	if cfg.Web.HTTPAddr != ":9000" || cfg.API.TCPAddr != ":9001" {
		t.Fatalf("unexpected listen addrs web=%s api=%s", cfg.Web.HTTPAddr, cfg.API.TCPAddr)
	}
	if cfg.Web.APICORSAllowOrigins != "https://fe.example.com" {
		t.Fatalf("unexpected cors origins: %s", cfg.Web.APICORSAllowOrigins)
	}
	if cfg.Web.MaxAPIBodyBytes != 2097152 {
		t.Fatalf("unexpected web max body bytes: %d", cfg.Web.MaxAPIBodyBytes)
	}
	if cfg.Web.DeviceListPage.DefaultSize != 30 || cfg.Web.DeviceListPage.MaxSize != 2000 {
		t.Fatalf("unexpected web page config: %+v", cfg.Web.DeviceListPage)
	}
	if cfg.Web.Metrics.MinValidTimestampMs != 1700000000000 || cfg.Web.Metrics.DefaultRangeLabel != "24h" {
		t.Fatalf("unexpected web metrics config: %+v", cfg.Web.Metrics)
	}
	if cfg.Web.LoginProtection.MaxFailures != 7 || cfg.Web.LoginProtection.Window != 20*time.Minute || cfg.Web.LoginProtection.Lockout != 45*time.Minute {
		t.Fatalf("unexpected login protection config: %+v", cfg.Web.LoginProtection)
	}
	if cfg.API.ReadTimeout != 75*time.Second || cfg.API.RegisterAckGraceDelay != 250*time.Millisecond {
		t.Fatalf("unexpected api config: %+v", cfg.API)
	}
	if cfg.Auth.RootURL != "https://iot.example.com" || cfg.Auth.CookieSecure {
		t.Fatalf("unexpected auth config: %+v", cfg.Auth)
	}
	if cfg.Auth.SessionCookieMaxAgeSeconds != 120 || cfg.Auth.RememberCookieMaxAgeSeconds != 86400 {
		t.Fatalf("unexpected auth cookie ages: %+v", cfg.Auth)
	}
	if cfg.Auth.GitHubClientID != "gh_id" || cfg.Auth.GitHubClientSecret != "gh_secret" {
		t.Fatalf("unexpected github auth config: %+v", cfg.Auth)
	}
	if cfg.Captcha.Provider != "turnstile" || cfg.Captcha.SiteKey != "site_key" || cfg.Captcha.SecretKey != "secret_key" || cfg.Captcha.VerifyTimeout != 3*time.Second {
		t.Fatalf("unexpected captcha config: %+v", cfg.Captcha)
	}
	if cfg.DeviceManager.QueueCapacity != 256 || cfg.DeviceManager.HeartbeatDeadline != 90*time.Second {
		t.Fatalf("unexpected device manager config: %+v", cfg.DeviceManager)
	}
	if cfg.DeviceManager.ExternalListPage.DefaultSize != 50 || cfg.DeviceManager.ExternalListPage.MaxSize != 500 {
		t.Fatalf("unexpected dm list page config: %+v", cfg.DeviceManager.ExternalListPage)
	}
	if cfg.DeviceManager.ExternalObservationLimit.Default != 2000 || cfg.DeviceManager.ExternalObservationLimit.Max != 30000 {
		t.Fatalf("unexpected dm obs limit config: %+v", cfg.DeviceManager.ExternalObservationLimit)
	}
	if cfg.Logger.Level != "debug" || cfg.Logger.Format != "json" || !cfg.Logger.AddSource || cfg.Logger.Service != "iot-backend" || cfg.Logger.Env != "test" {
		t.Fatalf("unexpected logger config: %+v", cfg.Logger)
	}
}

func TestResolveCookieSecureFallback(t *testing.T) {
	if !ResolveCookieSecure("", "production", "http://localhost:8080") {
		t.Fatal("production should default secure cookie to true")
	}
	if !ResolveCookieSecure("", "dev", "https://example.com") {
		t.Fatal("https root url should default secure cookie to true")
	}
	if ResolveCookieSecure("", "dev", "http://localhost:8080") {
		t.Fatal("dev + http should default secure cookie to false")
	}
}

func TestLoadPostgresDBConfig(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DB_PATH", "")
	t.Setenv("DB_DSN", "postgres://iot:iot@localhost:5432/goster?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DB.Driver != "postgres" {
		t.Fatalf("unexpected db driver: %s", cfg.DB.Driver)
	}
	if cfg.DB.DSN != "postgres://iot:iot@localhost:5432/goster?sslmode=disable" {
		t.Fatalf("unexpected db dsn: %s", cfg.DB.DSN)
	}
	if cfg.DB.SchemaMode != defaultPostgresSchemaMode {
		t.Fatalf("unexpected postgres schema mode: %s", cfg.DB.SchemaMode)
	}
}
