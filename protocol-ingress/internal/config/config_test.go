package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadFromEnvAppliesEnvironmentOverrides(t *testing.T) {
	cfg, err := LoadFromEnv(mapLookup(map[string]string{
		"PROTOCOL_INGRESS_SERVICE_NAME":                        "protocol-ingress-test",
		"PROTOCOL_INGRESS_ENV":                                 "test",
		"PROTOCOL_INGRESS_INSTANCE_ID":                         "ingress-test-01",
		"PROTOCOL_INGRESS_HTTP_ADDR":                           "127.0.0.1:18090",
		"PROTOCOL_INGRESS_SHUTDOWN_TIMEOUT":                    "3s",
		"PROTOCOL_INGRESS_CORE_ENDPOINT":                       "http://core.test",
		"PROTOCOL_INGRESS_CORE_TIMEOUT":                        "2s",
		"PROTOCOL_INGRESS_CORE_TOKEN":                          "secret-token",
		"PROTOCOL_INGRESS_CUSTOM_TCP_ENABLED":                  "true",
		"PROTOCOL_INGRESS_CUSTOM_TCP_ADDR":                     "127.0.0.1:19091",
		"PROTOCOL_INGRESS_CUSTOM_TCP_READ_TIMEOUT":             "30s",
		"PROTOCOL_INGRESS_CUSTOM_TCP_REGISTER_ACK_GRACE_DELAY": "5ms",
		"PROTOCOL_INGRESS_CUSTOM_TCP_DOWNLINK_MAX_BATCH":       "2",
	}))
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}
	if cfg.Service.Name != "protocol-ingress-test" || cfg.Service.Env != "test" || cfg.Service.InstanceID != "ingress-test-01" {
		t.Fatalf("unexpected service config: %+v", cfg.Service)
	}
	if cfg.Server.HTTPAddr != "127.0.0.1:18090" || cfg.Server.ShutdownTimeout != 3*time.Second {
		t.Fatalf("unexpected server config: %+v", cfg.Server)
	}
	if cfg.Core.Endpoint != "http://core.test" || cfg.Core.Timeout != 2*time.Second || cfg.Core.Token != "secret-token" {
		t.Fatalf("unexpected core config: %+v", cfg.Core)
	}
	if !cfg.Adapters.CustomTCP.Enabled || cfg.Adapters.CustomTCP.ListenAddr != "127.0.0.1:19091" || cfg.Adapters.CustomTCP.ReadTimeout != 30*time.Second || cfg.Adapters.CustomTCP.RegisterAckGraceDelay != 5*time.Millisecond || cfg.Adapters.CustomTCP.DownlinkMaxBatch != 2 {
		t.Fatalf("unexpected custom tcp config: %+v", cfg.Adapters.CustomTCP)
	}
}

func TestLoadFromEnvSupportsCloudPortFallback(t *testing.T) {
	cfg, err := LoadFromEnv(mapLookup(map[string]string{
		"PROTOCOL_INGRESS_INSTANCE_ID":   "pod-1",
		"PROTOCOL_INGRESS_CORE_ENDPOINT": "http://core-api:8080",
		"PORT":                           "8099",
	}))
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}
	if cfg.Server.HTTPAddr != ":8099" {
		t.Fatalf("expected PORT fallback, got %q", cfg.Server.HTTPAddr)
	}
}

func TestLoadFromEnvRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "duration", env: map[string]string{"PROTOCOL_INGRESS_CORE_TIMEOUT": "nope"}, want: "PROTOCOL_INGRESS_CORE_TIMEOUT"},
		{name: "bool", env: map[string]string{"PROTOCOL_INGRESS_CUSTOM_TCP_ENABLED": "maybe"}, want: "PROTOCOL_INGRESS_CUSTOM_TCP_ENABLED"},
		{name: "int", env: map[string]string{"PROTOCOL_INGRESS_CUSTOM_TCP_DOWNLINK_MAX_BATCH": "0"}, want: "PROTOCOL_INGRESS_CUSTOM_TCP_DOWNLINK_MAX_BATCH"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadFromEnv(mapLookup(tc.env))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestValidateRejectsInvalidConfig(t *testing.T) {
	cfg := Default()
	cfg.Core.Timeout = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for zero core timeout")
	}

	cfg = Default()
	cfg.Adapters.CustomTCP.Enabled = true
	cfg.Adapters.CustomTCP.ListenAddr = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty tcp addr")
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := values[key]
		return v, ok
	}
}
