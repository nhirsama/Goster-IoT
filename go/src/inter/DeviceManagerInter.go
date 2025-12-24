package inter

// DeviceStatus 定义设备的逻辑在线状态
type DeviceStatus int

const (
	StatusOffline DeviceStatus = iota // 离线
	StatusOnline                      // 在线
	StatusDelayed                     // 延迟（心跳超过阈值但未完全判定为离线）
)

// Notice 定义了下发给终端的异步指令载荷 [cite: 23, 24]
type Notice struct {
	Type      int    `json:"type"` // 通知类型 (例如: 1-配置更新, 2-紧急制动) [cite: 24]
	Msg       string `json:"msg"`  // 详细载荷内容 [cite: 23, 24]
	Timestamp int64  `json:"ts"`   // 指令生成的时间戳
}

// DeviceManager 负责维护设备的生命周期状态并管理待下发指令队列
type DeviceManager interface {
	// [状态管理]

	// Heartbeat 记录并更新设备的最后活跃时间戳，用于判定在线状态 [cite: 24]
	Heartbeat(uuid string) error

	// GetStatus 根据最后活跃时间与预设阈值，实时计算设备的逻辑状态 (在线/延迟/离线) [cite: 18]
	GetStatus(uuid string) DeviceStatus

	// [指令通知栈管理]

	// EnqueueNotice 管理端调用此接口将指令压入特定设备的待分发队列 [cite: 24]
	// nType: 通知类型; msg: 详细指令内容 [cite: 24]
	EnqueueNotice(uuid string, nType int, msg string) error

	// PopNotice 在设备心跳请求 (/heartbeat) 时由服务端调用 [cite: 18]
	// 从队列中拉取并删除最旧的一条待处理指令，若无通知则 ok 返回 false [cite: 24]
	PopNotice(uuid string) (notice Notice, ok bool)

	// ClearNotices 清空指定设备的所有待处理指令
	ClearNotices(uuid string) error
}
