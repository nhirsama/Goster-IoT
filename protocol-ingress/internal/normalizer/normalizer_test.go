package normalizer

import (
	"context"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/adapter"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestNormalizeEventBuildsCanonicalModel(t *testing.T) {
	n := New("ingress-test")
	observed := time.UnixMilli(1000).UTC()
	value := 21.5
	event := adapter.AdapterEvent{
		AdapterName:     "custom_tcp",
		ProtocolName:    "goster-wy",
		ProtocolVersion: "1",
		Transport:       "stream",
		RemoteAddr:      "127.0.0.1:10000",
		LocalAddr:       "127.0.0.1:8081",
		TenantID:        "tenant-a",
		Identity:        adapter.Identity{Type: "uuid", Value: "dev-1"},
		UUID:            "dev-1",
		Kind:            "telemetry",
		OccurredAt:      observed,
		ReceivedAt:      observed.Add(time.Second),
		Metrics: []adapter.MetricPoint{{
			Name:             "temperature",
			Value:            adapter.Value{Number: &value},
			Unit:             "°C",
			ObservedAt:       observed,
			LegacyMetricType: 1,
		}},
		Raw:            []byte{1, 2, 3},
		RawContentType: "application/octet-stream",
		Attributes:     map[string]any{"key": "value"},
		Labels:         map[string]string{"l": "v"},
		Frame: adapter.FrameInfo{
			MagicNumber: 0x5759,
			CommandCode: 0x0101,
			KeyID:       1,
			Sequence:    9,
			PayloadLen:  3,
		},
	}

	out, err := n.NormalizeEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("NormalizeEvent failed: %v", err)
	}
	if out.EventId == "" || out.IdempotencyKey != out.EventId {
		t.Fatalf("event id/idempotency not set: %+v", out)
	}
	if out.Context.SourceInstance != "ingress-test" || out.Context.Transport != ingressv1.Transport_TRANSPORT_STREAM || out.Context.Frame.CommandCode != 0x0101 {
		t.Fatalf("unexpected context: %+v", out.Context)
	}
	if out.EventType != ingressv1.EventType_EVENT_TYPE_TELEMETRY || len(out.Metrics) != 1 {
		t.Fatalf("unexpected event type/metrics: %+v", out)
	}
	if got := out.Metrics[0].GetValue().GetNumberValue(); got != value {
		t.Fatalf("unexpected metric value: %v", got)
	}
	if out.Raw == nil || len(out.Raw.Body) != 3 || out.Extensions.GetFields()["key"].GetStringValue() != "value" {
		t.Fatalf("raw/extensions missing: raw=%+v ext=%+v", out.Raw, out.Extensions)
	}
}

func TestNormalizeEventIDDeterministic(t *testing.T) {
	n := New("ingress-test")
	event := adapter.AdapterEvent{
		AdapterName:  "a",
		ProtocolName: "p",
		Transport:    "stream",
		Kind:         "heartbeat",
		Identity:     adapter.Identity{Type: "uuid", Value: "dev-1"},
		OccurredAt:   time.Unix(10, 0).UTC(),
		ReceivedAt:   time.Unix(11, 0).UTC(),
		Frame:        adapter.FrameInfo{Sequence: 1, CommandCode: 2},
		Raw:          []byte("raw"),
	}
	first, err := n.NormalizeEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("NormalizeEvent first failed: %v", err)
	}
	second, err := n.NormalizeEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("NormalizeEvent second failed: %v", err)
	}
	if first.EventId != second.EventId {
		t.Fatalf("event id should be deterministic: %s != %s", first.EventId, second.EventId)
	}
}

func TestNormalizeCommandMapsRawPayloadAndOptions(t *testing.T) {
	n := New("ingress-test")
	opts, err := structpb.NewStruct(map[string]any{"command_code": float64(0x0201), "qos": "1"})
	if err != nil {
		t.Fatalf("NewStruct: %v", err)
	}
	cmd := &ingressv1.CanonicalCommand{
		CommandId:           42,
		CommandUuid:         "cmd-42",
		TenantId:            "tenant-a",
		Uuid:                "dev-1",
		TargetIdentity:      &ingressv1.DeviceIdentity{Type: "uuid", Value: "dev-1"},
		Operation:           "config_push",
		ProtocolCommandCode: 0x0201,
		Payload:             &ingressv1.RawPayload{ContentType: "application/json", Body: []byte(`{"a":1}`)},
		Timeout:             durationpb.New(2 * time.Second),
		MaxAttempts:         3,
		AdapterOptions:      opts,
		Properties:          map[string]string{"p": "v"},
	}
	out, err := n.NormalizeCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("NormalizeCommand failed: %v", err)
	}
	if out.CommandID != 42 || out.ProtocolCommandCode != 0x0201 || out.Operation != "config_push" || string(out.Payload) != `{"a":1}` {
		t.Fatalf("unexpected command: %+v", out)
	}
	if out.Options["qos"] != "1" || out.Timeout != 2*time.Second || out.MaxAttempts != 3 {
		t.Fatalf("unexpected command options: %+v", out)
	}
}
