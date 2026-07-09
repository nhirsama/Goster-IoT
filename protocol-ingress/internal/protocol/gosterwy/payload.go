package gosterwy

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/adapter"
)

func ParseRegistrationPayload(payload string) (adapter.DeviceDescriptor, error) {
	parts := strings.Split(payload, "\x1e")
	if len(parts) < 6 {
		return adapter.DeviceDescriptor{}, fmt.Errorf("registration payload invalid: expected 6 fields, got %d", len(parts))
	}
	return adapter.DeviceDescriptor{
		Name:            parts[0],
		SerialNumber:    parts[1],
		MACAddress:      parts[2],
		HardwareVersion: parts[3],
		SoftwareVersion: parts[4],
		ConfigVersion:   parts[5],
		Identities: []adapter.Identity{
			{Type: "serial_number", Value: parts[1]},
			{Type: "mac", Value: parts[2]},
		},
		Labels: map[string]string{"adapter_protocol": "goster-wy"},
	}, nil
}

func ParseMetricsPayload(payload []byte) ([]adapter.MetricPoint, error) {
	if len(payload) < 17 {
		return nil, fmt.Errorf("payload too short")
	}
	startTimestamp := int64(binary.LittleEndian.Uint64(payload[0:8]))
	sampleInterval := binary.LittleEndian.Uint32(payload[8:12])
	dataType := payload[12]
	count := binary.LittleEndian.Uint32(payload[13:17])
	dataBlob := payload[17:]

	name, unit, ok := legacyMetricName(dataType)
	if !ok {
		return nil, fmt.Errorf("unsupported metrics data type: %d", dataType)
	}
	const pointSize = 4
	expectedLen := int(count) * pointSize
	if len(dataBlob) != expectedLen {
		return nil, fmt.Errorf("metrics blob length mismatch: expect %d, got %d", expectedLen, len(dataBlob))
	}

	points := make([]adapter.MetricPoint, 0, count)
	for i := 0; i < int(count); i++ {
		bits := binary.LittleEndian.Uint32(dataBlob[i*pointSize : (i+1)*pointSize])
		value32 := math.Float32frombits(bits)
		value := float64(value32)
		observedAt := time.UnixMilli(startTimestamp + int64(i)*int64(sampleInterval)).UTC()
		points = append(points, adapter.MetricPoint{
			Name:             name,
			Value:            adapter.Value{Number: &value},
			Unit:             unit,
			ObservedAt:       observedAt,
			LegacyMetricType: uint32(dataType),
		})
	}
	return points, nil
}

func ParseLogPayload(payload []byte) (adapter.LogRecord, error) {
	if len(payload) < 11 {
		return adapter.LogRecord{}, fmt.Errorf("log payload too short")
	}
	ts := int64(binary.LittleEndian.Uint64(payload[0:8]))
	level := logLevel(payload[8])
	msgLen := int(binary.LittleEndian.Uint16(payload[9:11]))
	if len(payload) < 11+msgLen {
		return adapter.LogRecord{}, fmt.Errorf("log message truncated")
	}
	return adapter.LogRecord{
		Level:      level,
		Message:    string(payload[11 : 11+msgLen]),
		Namespace:  "device",
		ObservedAt: time.UnixMilli(ts).UTC(),
	}, nil
}

func legacyMetricName(dataType uint8) (name string, unit string, ok bool) {
	switch dataType {
	case 1:
		return "temperature", "°C", true
	case 2:
		return "humidity", "%", true
	case 4:
		return "illuminance", "lx", true
	case 8:
		return "access_signal_a", "", true
	case 16:
		return "access_signal_b", "", true
	default:
		return "", "", false
	}
}

func logLevel(level uint8) string {
	switch level {
	case 0:
		return "debug"
	case 1:
		return "info"
	case 2:
		return "warn"
	case 3:
		return "error"
	default:
		return "info"
	}
}
