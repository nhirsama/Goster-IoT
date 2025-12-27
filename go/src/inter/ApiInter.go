package inter

// Api 定义了设备接入服务（Goster-WY 协议）的接口
// 它负责启动 TCP 监听，并处理所有设备侧的请求
type Api interface {
	// Start 启动 API 服务监听
	Start()

	// 以下为内部逻辑接口，虽暴露在接口中，但主要由 Start() 内部循环调用
	// 在单元测试中也可以单独测试这些逻辑

	// Handshake 处理冷启动握手
	Handshake(uuid string, token string) (sessionID string, err error)

	// Heartbeat 处理智能心跳
	Heartbeat(uuid string) (hasPending bool, err error)

	// UploadMetrics 处理采样数据上报
	UploadMetrics(uuid string, data MetricsUploadData) error

	// UploadLog 处理日志上报
	UploadLog(uuid string, level string, message string) error

	// GetMessages 获取下行消息
	GetMessages(uuid string) ([]interface{}, error)
}

// MetricsUploadData 对应协议中的 Payload 结构
type MetricsUploadData struct {
	StartTimestamp int64  // 采样开始时间 (Unix MS)
	SampleInterval uint32 // 采样间隔 (微秒)
	DataType       uint8  // 数据类型枚举
	Count          uint32 // 数据点数量
	DataBlob       []byte // 原始采样数据流
}
