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
		Topic:      "goster/v1/tenant-a/dev-1/telemetry",
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
	if event.Kind != "telemetry" || event.UUID != "dev-1" || event.TenantID != "tenant-a" {
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
		Topic:      "goster/v1/tenant-a/dev-1/ack",
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
	got := renderDownlinkTopic("goster/v1/{tenant}/{uuid}/downlink", deviceSession{UUID: "dev-1", TenantID: "tenant-a"}, adapterCommand("dev-override", "tenant-b"))
	if got != "goster/v1/tenant-b/dev-override/downlink" {
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

func adapterCommand(uuid, tenantID string) adapter.AdapterCommand {
	return adapter.AdapterCommand{UUID: uuid, TenantID: tenantID}
}
