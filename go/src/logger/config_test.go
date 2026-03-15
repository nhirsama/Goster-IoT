package logger

import "testing"

func TestConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "")
	t.Setenv("LOG_ADD_SOURCE", "")
	t.Setenv("LOG_SERVICE", "")
	t.Setenv("LOG_ENV", "")
	t.Setenv("APP_ENV", "")

	cfg := ConfigFromEnv()
	if cfg.Level != "info" {
		t.Fatalf("unexpected level: %s", cfg.Level)
	}
	if cfg.Format != "text" {
		t.Fatalf("unexpected format: %s", cfg.Format)
	}
	if cfg.Service != "goster-iot" {
		t.Fatalf("unexpected service: %s", cfg.Service)
	}
	if cfg.Env != "dev" {
		t.Fatalf("unexpected env: %s", cfg.Env)
	}
}

func TestConfigFromEnvOverride(t *testing.T) {
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_ADD_SOURCE", "true")
	t.Setenv("LOG_SERVICE", "iot-backend")
	t.Setenv("LOG_ENV", "")
	t.Setenv("APP_ENV", "prod")

	cfg := ConfigFromEnv()
	if cfg.Level != "debug" {
		t.Fatalf("unexpected level: %s", cfg.Level)
	}
	if cfg.Format != "json" {
		t.Fatalf("unexpected format: %s", cfg.Format)
	}
	if !cfg.AddSource {
		t.Fatal("add source should be true")
	}
	if cfg.Service != "iot-backend" {
		t.Fatalf("unexpected service: %s", cfg.Service)
	}
	if cfg.Env != "prod" {
		t.Fatalf("unexpected env: %s", cfg.Env)
	}
}

func TestNormalizeConfigFallback(t *testing.T) {
	cfg := normalizeConfig(Config{
		Level:   "invalid",
		Format:  "invalid",
		Service: "",
		Env:     "",
	})
	if cfg.Level != "info" || cfg.Format != "text" {
		t.Fatalf("unexpected normalize result: %#v", cfg)
	}
}
