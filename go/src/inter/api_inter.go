package inter

// api 定义了设备接入服务（Goster-WY 协议）的接口
// 它负责启动 TCP 监听，并处理所有设备侧的请求

// MetricsUploadData 对应协议中的 Payload 结构
type MetricsUploadData struct {
	StartTimestamp int64  // 采样开始时间 (Unix MS)
	SampleInterval uint32 // 采样间隔 (微秒)
	DataType       uint8  // 数据类型枚举
	Count          uint32 // 数据点数量
	DataBlob       []byte // 原始采样数据流
}

// LogLevel 日志级别枚举
type LogLevel uint8

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// LogUploadData 对应 CmdLogReport 的 Payload 结构
// 二进制布局: [Timestamp(8B)] + [Level(1B)] + [MsgLen(2B)] + [Message(N Bytes)]
type LogUploadData struct {
	Timestamp int64    // 日志产生时间 (Unix MS)
	Level     LogLevel // 日志级别
	Message   string   // 日志内容
}

type Api interface {
	// Start 启动 API 服务监听 (阻塞调用)
	Start()
}
