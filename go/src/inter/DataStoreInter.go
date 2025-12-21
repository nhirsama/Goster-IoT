package inter

import "time"

// DeviceMetadata 设备静态元数据
type DeviceMetadata struct {
	Name          string    `json:"name"`           // 设备名称
	HWVersion     string    `json:"hw_version"`     // 硬件版本
	SWVersion     string    `json:"sw_version"`     // 固件/软件版本
	ConfigVersion string    `json:"config_version"` // 配置文件版本
	SerialNumber  string    `json:"sn"`             // 序列号
	CreatedAt     time.Time `json:"created_at"`     // 首次注册时间
}

// DeviceRecord 设备记录（用于列表展示）
type DeviceRecord struct {
	UUID string         `json:"uuid"`
	Meta DeviceMetadata `json:"meta"`
}

// MetricPoint 传感器采样点
type MetricPoint struct {
	Timestamp int64   `json:"ts"`    // Unix 时间戳
	Value     float32 `json:"value"` // 物理数值
}

// DataStore 数据存储模块标准接口
type DataStore interface {
	// [生命周期]
	InitDevice(uuid string, meta DeviceMetadata) error
	DestroyDevice(uuid string) error

	// [配置与元数据管理]
	LoadConfig(uuid string, out interface{}) error
	SaveConfig(uuid string, data interface{}) error
	GetMetadata(uuid string) (DeviceMetadata, error)
	ListDevices(page, size int) ([]DeviceRecord, error)

	// [时序数据管理]
	AppendMetric(uuid string, ts int64, value float32) error
	QueryMetrics(uuid string, start, end int64) ([]MetricPoint, error)

	// [日志管理]
	WriteLog(uuid string, level string, message string) error
}
