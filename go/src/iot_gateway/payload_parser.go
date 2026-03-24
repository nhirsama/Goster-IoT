package iot_gateway

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// parseRegistrationPayload 解析设备注册报文。
func parseRegistrationPayload(payload string) (inter.DeviceMetadata, error) {
	parts := strings.Split(payload, "\x1e")
	if len(parts) < 6 {
		return inter.DeviceMetadata{}, fmt.Errorf("registration payload invalid: expected 6 fields, got %d", len(parts))
	}

	return inter.DeviceMetadata{
		Name:          parts[0],
		SerialNumber:  parts[1],
		MACAddress:    parts[2],
		HWVersion:     parts[3],
		SWVersion:     parts[4],
		ConfigVersion: parts[5],
	}, nil
}

// parseMetricsPayload 把设备原始指标载荷转换成平台统一采样点。
func parseMetricsPayload(payload []byte) ([]inter.MetricPoint, error) {
	if len(payload) < 17 {
		return nil, fmt.Errorf("payload too short")
	}

	data := inter.MetricsUploadData{
		StartTimestamp: int64(binary.LittleEndian.Uint64(payload[0:8])),
		SampleInterval: binary.LittleEndian.Uint32(payload[8:12]),
		DataType:       payload[12],
		Count:          binary.LittleEndian.Uint32(payload[13:17]),
		DataBlob:       payload[17:],
	}

	if data.DataType != 1 && data.DataType != 2 && data.DataType != 4 {
		return nil, fmt.Errorf("unsupported metrics data type: %d", data.DataType)
	}

	const pointSize = 4
	expectedLen := int(data.Count) * pointSize
	if len(data.DataBlob) != expectedLen {
		return nil, fmt.Errorf("metrics blob length mismatch: expect %d, got %d", expectedLen, len(data.DataBlob))
	}

	points := make([]inter.MetricPoint, 0, data.Count)
	for i := 0; i < int(data.Count); i++ {
		bits := binary.LittleEndian.Uint32(data.DataBlob[i*pointSize : (i+1)*pointSize])
		offsetMs := int64(i) * int64(data.SampleInterval)
		points = append(points, inter.MetricPoint{
			Timestamp: data.StartTimestamp + offsetMs,
			Value:     math.Float32frombits(bits),
			Type:      data.DataType,
		})
	}

	return points, nil
}

// parseLogPayload 解析设备日志载荷。
func parseLogPayload(payload []byte) (inter.LogUploadData, error) {
	if len(payload) < 11 {
		return inter.LogUploadData{}, fmt.Errorf("log payload too short")
	}

	ts := int64(binary.LittleEndian.Uint64(payload[0:8]))
	levelVal := inter.LogLevel(payload[8])
	msgLen := int(binary.LittleEndian.Uint16(payload[9:11]))
	if len(payload) < 11+msgLen {
		return inter.LogUploadData{}, fmt.Errorf("log message truncated")
	}

	return inter.LogUploadData{
		Timestamp: ts,
		Level:     levelVal,
		Message:   string(payload[11 : 11+msgLen]),
	}, nil
}
