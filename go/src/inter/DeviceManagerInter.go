package inter

// DeviceStatus 定义设备的逻辑在线状态
type DeviceStatus int

const (
	StatusOffline DeviceStatus = iota // 离线
	StatusOnline                      // 在线
	StatusDelayed                     // 延迟（心跳超过阈值但未完全判定为离线）
)

// DeviceManager 定义设备管理的核心业务逻辑接口
type DeviceManager interface {

	// HandleHeartbeat 处理设备心跳
	// 作用:
	// 1. 更新设备在线状态/最后活跃时间
	// 2. 检查消息队列，如果有堆积的指令，通过返回值带回给设备
	// 返回: (待下发的指令, 错误信息)
	HandleHeartbeat(uuid string)

	QueryDeviceStatus(uuid string) (DeviceStatus, error)
}

// MessageQueue 定义消息队列的底层操作接口
// 用于缓冲后端发往设备的指令
type MessageQueue interface {
	// Push 入队
	// 将指令推入指定 UUID 的队列中
	Push(uuid string, message interface{}) error

	// Pop 出队
	// 从指定 UUID 的队列中取出最早的一条指令 (FIFO)
	// 返回: (指令内容, 是否存在指令)
	Pop(uuid string) (interface{}, bool)
}
