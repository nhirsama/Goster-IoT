package device_manager

import (
	"fmt"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// TelemetryIngestService 负责把设备解析后的指标、日志、事件沉淀到数据存储。
type TelemetryIngestService struct {
	dataStore inter.TelemetryStore
}

// NewTelemetryIngestService 创建默认遥测接收服务。
func NewTelemetryIngestService(ds inter.TelemetryStore) inter.TelemetryIngestService {
	return &TelemetryIngestService{dataStore: ds}
}

// IngestMetrics 批量写入设备指标。
func (s *TelemetryIngestService) IngestMetrics(uuid string, points []inter.MetricPoint) error {
	return s.dataStore.BatchAppendMetrics(uuid, points)
}

// IngestLog 写入设备运行日志。
func (s *TelemetryIngestService) IngestLog(uuid string, data inter.LogUploadData) error {
	finalMsg := fmt.Sprintf("[%s] %s", time.UnixMilli(data.Timestamp).Format(time.DateTime), data.Message)
	return s.dataStore.WriteLog(uuid, mapTelemetryLogLevel(data.Level), finalMsg)
}

// IngestEvent 写入设备事件。
func (s *TelemetryIngestService) IngestEvent(uuid string, payload []byte) error {
	return s.dataStore.WriteLog(uuid, "EVENT", string(payload))
}

// IngestDeviceError 写入设备错误日志。
func (s *TelemetryIngestService) IngestDeviceError(uuid string, payload []byte) error {
	return s.dataStore.WriteLog(uuid, "ERROR", string(payload))
}

func mapTelemetryLogLevel(level inter.LogLevel) string {
	switch level {
	case inter.LogLevelDebug:
		return "DEBUG"
	case inter.LogLevelInfo:
		return "INFO"
	case inter.LogLevelWarn:
		return "WARN"
	case inter.LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
