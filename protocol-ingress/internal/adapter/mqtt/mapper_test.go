package mqtt

import (
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/adapter"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
)

func TestMapperMapsGosterTelemetryTopic(t *testing.T) {
	cfg := config.Default().Adapters.MQTT
	mapper := NewMapper(cfg)
	msg := InboundMessage{
		Topic:      "goster/v1/dev-1/telemetry",
		Payload:    []byte(`{"token":"tok-1","temperature":25.5,"humidity":60,"ts":1700000000000}`),
		QoS:        1,
		ReceivedAt: time.Unix(1700000001, 0).UTC(),
	}

	out, err := mapper.Map(msg)
	if err != nil {
		t.Fatalf("Map failed: %v", err)
	}
	if out.Token != "tok-1" {
		t.Fatalf("unexpected token: %q", out.Token)
	}
	event := out.Event
	if event.Kind != "telemetry" || event.UUID != "dev-1" || event.TenantID != "" {
		t.Fatalf("unexpected event identity: %+v", event)
	}
	if len(event.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %+v", event.Metrics)
	}
	if event.Metrics[0].Name != "temperature" || *event.Metrics[0].Value.Number != 25.5 || event.Metrics[0].LegacyMetricType != 1 {
		t.Fatalf("unexpected first metric: %+v", event.Metrics[0])
	}
	if event.Metrics[1].Name != "humidity" || event.Metrics[1].LegacyMetricType != 2 {
		t.Fatalf("unexpected second metric: %+v", event.Metrics[1])
	}
}

func TestMapperMapsZigbee2MQTTStatePayload(t *testing.T) {
	cfg := config.Default().Adapters.MQTT
	mapper := NewMapper(cfg)
	msg := InboundMessage{
		Topic:      "zigbee2mqtt/livingroom_sensor",
		Payload:    []byte(`{"temperature":23.7,"humidity":55.2,"battery":91,"occupancy":true,"linkquality":102}`),
		QoS:        0,
		Retained:   true,
		ReceivedAt: time.Unix(1700000001, 0).UTC(),
	}

	out, err := mapper.Map(msg)
	if err != nil {
		t.Fatalf("Map failed: %v", err)
	}
	event := out.Event
	if event.ProtocolName != "zigbee2mqtt" || event.UUID == "" || event.Identity.Value != "livingroom_sensor" {
		t.Fatalf("unexpected zigbee identity: %+v", event)
	}
	if len(event.Metrics) != 4 {
		t.Fatalf("expected 4 metrics, got %+v", event.Metrics)
	}
	if len(event.States) != 1 || event.States[0].Name != "occupancy" || event.States[0].Value.Bool == nil || !*event.States[0].Value.Bool {
		t.Fatalf("unexpected states: %+v", event.States)
	}
	if event.Labels["mqtt_retained"] != "true" {
		t.Fatalf("expected retained label, got %+v", event.Labels)
	}
}

func TestMapperMapsCommandAck(t *testing.T) {
	mapper := NewMapper(config.Default().Adapters.MQTT)
	out, err := mapper.Map(InboundMessage{
		Topic:      "goster/v1/dev-1/ack",
		Payload:    []byte(`{"command_id":42,"command_uuid":"cmd-42","status":"acked","operation":"set"}`),
		ReceivedAt: time.Unix(1700000001, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("Map failed: %v", err)
	}
	if out.Event.Kind != "command_ack" || out.Event.Receipt == nil {
		t.Fatalf("unexpected ack event: %+v", out.Event)
	}
	if out.Event.Receipt.CommandID != 42 || out.Event.Receipt.CommandUUID != "cmd-42" || out.Event.Receipt.Status != "acked" {
		t.Fatalf("unexpected receipt: %+v", out.Event.Receipt)
	}
}

func TestRenderDownlinkTopic(t *testing.T) {
	got := renderDownlinkTopic("goster/v1/{uuid}/downlink", deviceSession{UUID: "dev-1", TenantID: "tenant-a"}, adapterCommand("dev-override", "tenant-b"))
	if got != "goster/v1/dev-override/downlink" {
		t.Fatalf("unexpected topic: %s", got)
	}
}

func TestMapperSkipsZigbee2MQTTBridgeTopic(t *testing.T) {
	mapper := NewMapper(config.Default().Adapters.MQTT)
	_, err := mapper.Map(InboundMessage{Topic: "zigbee2mqtt/bridge/devices", Payload: []byte(`[]`)})
	if err == nil {
		t.Fatal("expected bridge topic error")
	}
}

func TestMapperMapsOfficialZigbee2MQTTExposes(t *testing.T) {
	mapper := NewMapper(config.Default().Adapters.MQTT)
	cases := []struct {
		name        string
		topic       string
		payload     string
		wantMetrics []string
		wantStates  []string
	}{
		{
			name:        "aqara_temperature_humidity_pressure",
			topic:       "zigbee2mqtt/aqara_thp",
			payload:     `{"battery":91,"humidity":55.2,"linkquality":102,"pressure":1012.7,"temperature":23.7,"voltage":3005}`,
			wantMetrics: []string{"temperature", "humidity", "battery", "linkquality", "voltage", "pressure"},
		},
		{
			name:        "tuya_power_plug",
			topic:       "zigbee2mqtt/washer_plug",
			payload:     `{"current":0.21,"energy":1.34,"linkquality":125,"power":25.5,"state":"ON","voltage":230.1}`,
			wantMetrics: []string{"current", "energy", "linkquality", "power", "voltage"},
			wantStates:  []string{"state"},
		},
		{
			name:        "leak_smoke_and_action_states",
			topic:       "zigbee2mqtt/safety_cluster",
			payload:     `{"water_leak":true,"smoke":false,"action":"single","battery_low":false,"linkquality":88}`,
			wantMetrics: []string{"linkquality"},
			wantStates:  []string{"water_leak", "smoke", "action", "battery_low"},
		},
		{
			name:        "light_state",
			topic:       "zigbee2mqtt/desk_light",
			payload:     `{"state":"ON","brightness":180,"color_temp":370,"linkquality":140}`,
			wantMetrics: []string{"linkquality"},
			wantStates:  []string{"state", "brightness", "color_temp"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := mapper.Map(InboundMessage{
				Topic:      tc.topic,
				Payload:    []byte(tc.payload),
				QoS:        1,
				ReceivedAt: time.Unix(1700000001, 0).UTC(),
			})
			if err != nil {
				t.Fatalf("Map failed: %v", err)
			}
			for _, metricName := range tc.wantMetrics {
				if !hasMetric(out.Event.Metrics, metricName) {
					t.Fatalf("missing metric %q in %+v", metricName, out.Event.Metrics)
				}
			}
			for _, stateName := range tc.wantStates {
				if !hasState(out.Event.States, stateName) {
					t.Fatalf("missing state %q in %+v", stateName, out.Event.States)
				}
			}
		})
	}
}

func TestMapperMapsZigbee2MQTTAvailabilityAndMalformedPayload(t *testing.T) {
	mapper := NewMapper(config.Default().Adapters.MQTT)
	availability, err := mapper.Map(InboundMessage{
		Topic:      "zigbee2mqtt/front_door/availability",
		Payload:    []byte(`offline`),
		ReceivedAt: time.Unix(1700000001, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("Map availability failed: %v", err)
	}
	if availability.Event.Kind != "availability" || availability.Event.Availability != "offline" {
		t.Fatalf("unexpected availability event: %+v", availability.Event)
	}

	raw, err := mapper.Map(InboundMessage{
		Topic:      "zigbee2mqtt/bad_json_sensor",
		Payload:    []byte(`{"temperature":`),
		ReceivedAt: time.Unix(1700000001, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("Map malformed payload failed: %v", err)
	}
	if raw.Event.Kind != "event" || string(raw.Event.Raw) != `{"temperature":` {
		t.Fatalf("unexpected malformed payload mapping: %+v", raw.Event)
	}
}

func adapterCommand(uuid, tenantID string) adapter.AdapterCommand {
	return adapter.AdapterCommand{UUID: uuid, TenantID: tenantID}
}

func hasMetric(metrics []adapter.MetricPoint, name string) bool {
	for _, m := range metrics {
		if m.Name == name {
			return true
		}
	}
	return false
}

func hasState(states []adapter.StatePoint, name string) bool {
	for _, s := range states {
		if s.Name == name {
			return true
		}
	}
	return false
}
