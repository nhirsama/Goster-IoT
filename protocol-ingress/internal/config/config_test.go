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
		"PROTOCOL_INGRESS_CUSTOM_TCP_IDLE_TIMEOUT":             "90s",
		"PROTOCOL_INGRESS_CUSTOM_TCP_RPC_TIMEOUT":              "750ms",
		"PROTOCOL_INGRESS_CUSTOM_TCP_REGISTER_ACK_GRACE_DELAY": "5ms",
		"PROTOCOL_INGRESS_CUSTOM_TCP_DOWNLINK_MAX_BATCH":       "2",
		"PROTOCOL_INGRESS_MQTT_ENABLED":                        "true",
		"PROTOCOL_INGRESS_MQTT_MODE":                           "embedded",
		"PROTOCOL_INGRESS_MQTT_LISTEN_ADDR":                    "127.0.0.1:18883",
		"PROTOCOL_INGRESS_MQTT_AUTH_MODE":                      "client_password_token",
		"PROTOCOL_INGRESS_MQTT_BROKER_URL":                     "ssl://mqtt.test:8883",
		"PROTOCOL_INGRESS_MQTT_CLIENT_ID":                      "ingress-mqtt-test",
		"PROTOCOL_INGRESS_MQTT_USERNAME":                       "mqtt-user",
		"PROTOCOL_INGRESS_MQTT_PASSWORD":                       "mqtt-pass",
		"PROTOCOL_INGRESS_MQTT_SUBSCRIBE_TOPICS":               "goster/v1/+/+/telemetry, goster/v1/+/+/ack",
		"PROTOCOL_INGRESS_MQTT_QOS":                            "2",
		"PROTOCOL_INGRESS_MQTT_CONNECT_TIMEOUT":                "4s",
		"PROTOCOL_INGRESS_MQTT_KEEP_ALIVE":                     "45s",
		"PROTOCOL_INGRESS_MQTT_MESSAGE_BUFFER":                 "32",
		"PROTOCOL_INGRESS_MQTT_RPC_TIMEOUT":                    "900ms",
		"PROTOCOL_INGRESS_MQTT_BASE_TOPIC":                     "goster/v2",
		"PROTOCOL_INGRESS_MQTT_ZIGBEE2MQTT_BASE_TOPIC":         "z2m",
		"PROTOCOL_INGRESS_MQTT_SOURCE":                         "mqtt-test",
		"PROTOCOL_INGRESS_MQTT_DOWNLINK_ENABLED":               "true",
		"PROTOCOL_INGRESS_MQTT_DOWNLINK_TOPIC":                 "goster/v2/{tenant}/{uuid}/cmd",
		"PROTOCOL_INGRESS_MQTT_DOWNLINK_POLL_INTERVAL":         "3s",
		"PROTOCOL_INGRESS_MQTT_DOWNLINK_DEVICE_TTL":            "30s",
		"PROTOCOL_INGRESS_MQTT_DOWNLINK_MAX_BATCH":             "4",
		"PROTOCOL_INGRESS_MQTT_DOWNLINK_RETAINED":              "true",
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
	if !cfg.Adapters.CustomTCP.Enabled || cfg.Adapters.CustomTCP.ListenAddr != "127.0.0.1:19091" || cfg.Adapters.CustomTCP.ReadTimeout != 30*time.Second || cfg.Adapters.CustomTCP.IdleTimeout != 90*time.Second || cfg.Adapters.CustomTCP.RPCTimeout != 750*time.Millisecond || cfg.Adapters.CustomTCP.RegisterAckGraceDelay != 5*time.Millisecond || cfg.Adapters.CustomTCP.DownlinkMaxBatch != 2 {
		t.Fatalf("unexpected custom tcp config: %+v", cfg.Adapters.CustomTCP)
	}
	if !cfg.Adapters.MQTT.Enabled || cfg.Adapters.MQTT.Mode != "embedded" || cfg.Adapters.MQTT.ListenAddr != "127.0.0.1:18883" || cfg.Adapters.MQTT.AuthMode != "client_password_token" || cfg.Adapters.MQTT.BrokerURL != "ssl://mqtt.test:8883" || cfg.Adapters.MQTT.ClientID != "ingress-mqtt-test" || cfg.Adapters.MQTT.Username != "mqtt-user" || cfg.Adapters.MQTT.Password != "mqtt-pass" {
		t.Fatalf("unexpected mqtt identity config: %+v", cfg.Adapters.MQTT)
	}
	if len(cfg.Adapters.MQTT.SubscribeTopics) != 2 || cfg.Adapters.MQTT.SubscribeTopics[0] != "goster/v1/+/+/telemetry" || cfg.Adapters.MQTT.SubscribeTopics[1] != "goster/v1/+/+/ack" {
		t.Fatalf("unexpected mqtt topics: %+v", cfg.Adapters.MQTT.SubscribeTopics)
	}
	if cfg.Adapters.MQTT.QoS != 2 || cfg.Adapters.MQTT.ConnectTimeout != 4*time.Second || cfg.Adapters.MQTT.KeepAlive != 45*time.Second || cfg.Adapters.MQTT.MessageBuffer != 32 || cfg.Adapters.MQTT.RPCTimeout != 900*time.Millisecond || cfg.Adapters.MQTT.BaseTopic != "goster/v2" || cfg.Adapters.MQTT.Zigbee2MQTTBaseTopic != "z2m" || cfg.Adapters.MQTT.Source != "mqtt-test" {
		t.Fatalf("unexpected mqtt config: %+v", cfg.Adapters.MQTT)
	}
	if !cfg.Adapters.MQTT.DownlinkEnabled || cfg.Adapters.MQTT.DownlinkTopic != "goster/v2/{tenant}/{uuid}/cmd" || cfg.Adapters.MQTT.DownlinkPollInterval != 3*time.Second || cfg.Adapters.MQTT.DownlinkDeviceTTL != 30*time.Second || cfg.Adapters.MQTT.DownlinkMaxBatch != 4 || !cfg.Adapters.MQTT.DownlinkRetained {
		t.Fatalf("unexpected mqtt downlink config: %+v", cfg.Adapters.MQTT)
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

func TestLoadFromEnvSupportsSharedIngressTokenFallback(t *testing.T) {
	cfg, err := LoadFromEnv(mapLookup(map[string]string{
		"PROTOCOL_INGRESS_CORE_ENDPOINT": "http://core-api:8080",
		"PROTOCOL_INGRESS_TOKEN":         "shared-secret",
	}))
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}
	if cfg.Core.Token != "shared-secret" {
		t.Fatalf("expected shared PROTOCOL_INGRESS_TOKEN fallback, got %q", cfg.Core.Token)
	}

	cfg, err = LoadFromEnv(mapLookup(map[string]string{
		"PROTOCOL_INGRESS_CORE_ENDPOINT": "http://core-api:8080",
		"PROTOCOL_INGRESS_TOKEN":         "shared-secret",
		"PROTOCOL_INGRESS_CORE_TOKEN":    "core-specific-secret",
	}))
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}
	if cfg.Core.Token != "core-specific-secret" {
		t.Fatalf("expected PROTOCOL_INGRESS_CORE_TOKEN to override fallback, got %q", cfg.Core.Token)
	}
}

func TestLoadFromEnvRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "duration", env: map[string]string{"PROTOCOL_INGRESS_CORE_TIMEOUT": "nope"}, want: "PROTOCOL_INGRESS_CORE_TIMEOUT"},
		{name: "idle timeout", env: map[string]string{"PROTOCOL_INGRESS_CUSTOM_TCP_IDLE_TIMEOUT": "nope"}, want: "PROTOCOL_INGRESS_CUSTOM_TCP_IDLE_TIMEOUT"},
		{name: "rpc timeout", env: map[string]string{"PROTOCOL_INGRESS_CUSTOM_TCP_RPC_TIMEOUT": "0s"}, want: "PROTOCOL_INGRESS_CUSTOM_TCP_RPC_TIMEOUT"},
		{name: "bool", env: map[string]string{"PROTOCOL_INGRESS_CUSTOM_TCP_ENABLED": "maybe"}, want: "PROTOCOL_INGRESS_CUSTOM_TCP_ENABLED"},
		{name: "int", env: map[string]string{"PROTOCOL_INGRESS_CUSTOM_TCP_DOWNLINK_MAX_BATCH": "0"}, want: "PROTOCOL_INGRESS_CUSTOM_TCP_DOWNLINK_MAX_BATCH"},
		{name: "mqtt mode", env: map[string]string{"PROTOCOL_INGRESS_MQTT_MODE": "sideways"}, want: "PROTOCOL_INGRESS_MQTT_MODE"},
		{name: "mqtt auth mode", env: map[string]string{"PROTOCOL_INGRESS_MQTT_AUTH_MODE": "basic"}, want: "PROTOCOL_INGRESS_MQTT_AUTH_MODE"},
		{name: "mqtt qos", env: map[string]string{"PROTOCOL_INGRESS_MQTT_QOS": "3"}, want: "PROTOCOL_INGRESS_MQTT_QOS"},
		{name: "mqtt buffer", env: map[string]string{"PROTOCOL_INGRESS_MQTT_MESSAGE_BUFFER": "0"}, want: "PROTOCOL_INGRESS_MQTT_MESSAGE_BUFFER"},
		{name: "mqtt downlink poll", env: map[string]string{"PROTOCOL_INGRESS_MQTT_DOWNLINK_POLL_INTERVAL": "0s"}, want: "PROTOCOL_INGRESS_MQTT_DOWNLINK_POLL_INTERVAL"},
		{name: "mqtt downlink batch", env: map[string]string{"PROTOCOL_INGRESS_MQTT_DOWNLINK_MAX_BATCH": "0"}, want: "PROTOCOL_INGRESS_MQTT_DOWNLINK_MAX_BATCH"},
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
