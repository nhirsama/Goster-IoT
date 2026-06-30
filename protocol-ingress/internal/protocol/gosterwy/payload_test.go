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
