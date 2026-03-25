package inter

import "context"

// IoT Gateway 负责设备接入层网络通信。
// 当前实现仍与核心服务同进程运行，但接口已经按未来 gRPC 微服务边界抽象。

// MetricsUploadData 对应设备上报指标载荷的原始结构。
type MetricsUploadData struct {
	StartTimestamp int64  // 采样开始时间 (Unix MS)
	SampleInterval uint32 // 采样间隔 (毫秒)
	DataType       uint8  // 数据类型枚举
	Count          uint32 // 数据点数量
	DataBlob       []byte // 原始采样数据流
}

// LogLevel 日志级别枚举。
type LogLevel uint8

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// LogUploadData 对应 CmdLogReport 的解析结果。
// 二进制布局: [Timestamp(8B)] + [Level(1B)] + [MsgLen(2B)] + [Message(N Bytes)]
type LogUploadData struct {
	Timestamp int64    // 日志产生时间 (Unix MS)
	Level     LogLevel // 日志级别
	Message   string   // 日志内容
}

// RegistrationStatus 是设备注册流程在网络协议中的返回码。
type RegistrationStatus byte

const (
	RegistrationAccepted RegistrationStatus = 0x00
	RegistrationRejected RegistrationStatus = 0x01
	RegistrationPending  RegistrationStatus = 0x02
)

// DeviceRegistrationResult 描述一次设备注册申请的业务处理结果。
type DeviceRegistrationResult struct {
	Status RegistrationStatus
	UUID   string
	Token  string
}

// GatewayBackend 抽象 IoT Gateway 与核心服务之间的后端契约。
// 目前可由本地适配器实现，后续可直接替换为 gRPC 客户端实现。
type GatewayBackend interface {
	AuthenticateDevice(token string) (uuid string, err error)
	RegisterDevice(meta DeviceMetadata) (DeviceRegistrationResult, error)
	ReportHeartbeat(uuid string) error
	ReportMetrics(uuid string, points []MetricPoint) error
	ReportLog(uuid string, data LogUploadData) error
	ReportEvent(uuid string, payload []byte) error
	ReportDeviceError(uuid string, payload []byte) error
	PopDownlink(uuid string) (DownlinkMessage, bool, error)
	MarkDownlinkSent(commandID int64) error
	MarkDownlinkAcked(commandID int64) error
	MarkDownlinkFailed(commandID int64, errorText string) error
}

// IoTGateway 定义网络接入层服务接口。
type IoTGateway interface {
	// Start 启动设备接入服务监听，并在 ctx 取消时退出。
	Start(ctx context.Context) error
}
