package gosterwy

import (
	"encoding/binary"
	"math"
	"testing"
)

func buildMetricsPayload(start int64, interval uint32, dataType uint8, values ...float32) []byte {
	payload := make([]byte, 17+len(values)*4)
	binary.LittleEndian.PutUint64(payload[0:8], uint64(start))
	binary.LittleEndian.PutUint32(payload[8:12], interval)
	payload[12] = dataType
	binary.LittleEndian.PutUint32(payload[13:17], uint32(len(values)))
	for i, value := range values {
		binary.LittleEndian.PutUint32(payload[17+i*4:], math.Float32bits(value))
	}
	return payload
}

func buildLogPayload(ts int64, level byte, msg string) []byte {
	payload := make([]byte, 11+len(msg))
	binary.LittleEndian.PutUint64(payload[0:8], uint64(ts))
	payload[8] = level
	binary.LittleEndian.PutUint16(payload[9:11], uint16(len(msg)))
	copy(payload[11:], msg)
	return payload
}

func TestParseRegistrationPayload(t *testing.T) {
	dev, err := ParseRegistrationPayload("Device\x1eSN1\x1eAA:BB\x1ehw\x1esw\x1ecfg")
	if err != nil {
		t.Fatalf("ParseRegistrationPayload failed: %v", err)
	}
	if dev.Name != "Device" || dev.SerialNumber != "SN1" || dev.MACAddress != "AA:BB" || len(dev.Identities) != 2 {
		t.Fatalf("unexpected device: %+v", dev)
	}
	if _, err := ParseRegistrationPayload("bad"); err == nil {
		t.Fatal("expected invalid registration error")
	}
}

func TestParseMetricsPayload(t *testing.T) {
	points, err := ParseMetricsPayload(buildMetricsPayload(1000, 500, 1, 21.5, 22.25))
	if err != nil {
		t.Fatalf("ParseMetricsPayload failed: %v", err)
	}
	if len(points) != 2 || points[0].Name != "temperature" || points[0].Unit != "°C" || points[0].ObservedAt.UnixMilli() != 1000 || points[1].ObservedAt.UnixMilli() != 1500 {
		t.Fatalf("unexpected points: %+v", points)
	}
	if got := *points[0].Value.Number; got != float64(float32(21.5)) {
		t.Fatalf("unexpected value: %v", got)
	}
	if _, err := ParseMetricsPayload(buildMetricsPayload(1000, 500, 9, 1)); err == nil {
		t.Fatal("expected unsupported type error")
	}
	bad := buildMetricsPayload(1000, 500, 1, 1)
	if _, err := ParseMetricsPayload(bad[:len(bad)-1]); err == nil {
		t.Fatal("expected length mismatch")
	}
}

func TestParseMetricsPayloadSupportsAccessControlLegacyTypes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		dataType uint8
		value    float32
	}{
		{name: "input_type_8", dataType: 8, value: 1},
		{name: "input_type_16", dataType: 16, value: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			points, err := ParseMetricsPayload(buildMetricsPayload(2000, 250, tc.dataType, tc.value))
			if err != nil {
				t.Fatalf("ParseMetricsPayload type %d failed: %v", tc.dataType, err)
			}
			if len(points) != 1 {
				t.Fatalf("expected one point, got %+v", points)
			}
			point := points[0]
			if point.LegacyMetricType != uint32(tc.dataType) {
				t.Fatalf("legacy metric type mismatch: got=%d want=%d", point.LegacyMetricType, tc.dataType)
			}
			if point.Name == "" {
				t.Fatalf("access-control metric type %d should have a stable metric name", tc.dataType)
			}
			if point.ObservedAt.UnixMilli() != 2000 {
				t.Fatalf("unexpected observed time: %s", point.ObservedAt)
			}
			if point.Value.Number == nil || *point.Value.Number != float64(tc.value) {
				t.Fatalf("unexpected numeric value: %+v", point.Value.Number)
			}
		})
	}
}

func TestParseMetricsPayloadAccessControlSignals(t *testing.T) {
	signalA, err := ParseMetricsPayload(buildMetricsPayload(1000, 500, 8, 1))
	if err != nil {
		t.Fatalf("ParseMetricsPayload access signal A failed: %v", err)
	}
	if len(signalA) != 1 || signalA[0].Name != "access_signal_a" || signalA[0].LegacyMetricType != 8 {
		t.Fatalf("unexpected access signal A points: %+v", signalA)
	}

	signalB, err := ParseMetricsPayload(buildMetricsPayload(1200, 500, 16, 0))
	if err != nil {
		t.Fatalf("ParseMetricsPayload access signal B failed: %v", err)
	}
	if len(signalB) != 1 || signalB[0].Name != "access_signal_b" || signalB[0].LegacyMetricType != 16 {
		t.Fatalf("unexpected access signal B points: %+v", signalB)
	}
}

func TestParseLogPayload(t *testing.T) {
	record, err := ParseLogPayload(buildLogPayload(2000, 2, "battery low"))
	if err != nil {
		t.Fatalf("ParseLogPayload failed: %v", err)
	}
	if record.Level != "warn" || record.Message != "battery low" || record.ObservedAt.UnixMilli() != 2000 {
		t.Fatalf("unexpected log: %+v", record)
	}
	if _, err := ParseLogPayload([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected too short error")
	}
}
