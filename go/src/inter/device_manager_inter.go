package inter

import "errors"

// 定义鉴权相关的标准错误
var (
	ErrInvalidToken = errors.New("auth: 令牌无效或已过期")
	ErrAccessDenied = errors.New("auth: 该资源访问受限")

	// ErrDeviceRefused 对应 AuthenticateRefuse
	ErrDeviceRefused = errors.New("auth: 设备认证已被拒绝，禁止接入")

	// ErrDevicePending 对应 AuthenticatePending
	ErrDevicePending = errors.New("auth: 设备认证审核中，请等待管理员通过")

	// ErrDeviceUnknown 对应 AuthenticateUnknown
	ErrDeviceUnknown = errors.New("auth: 未找到对应设备信息或状态未知")
)

// DeviceStatus 定义设备的逻辑在线状态
type DeviceStatus int

const (
	StatusOffline DeviceStatus = iota // 离线
	StatusOnline                      // 在线
	StatusDelayed                     // 延迟（心跳超过阈值但未完全判定为离线）
)

// DeviceManager 定义设备管理的核心业务逻辑接口
// 合并了原有的 IdentityManager (身份与生命周期) 和 DeviceManager (运行时状态) 功能
type DeviceManager interface {

	// --- 身份与生命周期管理 (Identity & Lifecycle) ---

	// GenerateUUID 根据设备元数据生成唯一的 UUID
	GenerateUUID(meta DeviceMetadata) (uuid string)

	// RegisterDevice 注册新设备 (初始化 Pending 状态)
	RegisterDevice(meta DeviceMetadata) (err error)

	// Authenticate 验证 Token 合法性，返回 UUID
	Authenticate(token string) (uuid string, err error)

	// UpdateDeviceAuthenticateStatus 更新认证状态 (并处理 Token 变更)
	UpdateDeviceAuthenticateStatus(uuid string, Status AuthenticateStatusType) (token string, err error)

	// RefreshToken 刷新设备 Token
	RefreshToken(uuid string) (newToken string, err error)

	// RevokeToken 吊销设备 Token
	RevokeToken(uuid string) error

	// GetDeviceMetadata 获取设备详情
	GetDeviceMetadata(uuid string) (DeviceMetadata, error)

	// ApproveDevice 批准设备接入
	ApproveDevice(uuid string) error

	// RejectDevice 拒绝设备接入
	RejectDevice(uuid string) error

	// UnblockDevice 解除设备屏蔽
	UnblockDevice(uuid string) error

	// DeleteDevice 物理删除设备
	DeleteDevice(uuid string) error

	// --- 运行时状态管理 (Runtime State) ---

	// HandleHeartbeat 处理设备心跳
	HandleHeartbeat(uuid string)

	// QueryDeviceStatus 查询设备在线状态
	QueryDeviceStatus(uuid string) (DeviceStatus, error)

	// --- 消息队列 (Message Queue) ---

	// QueuePush 将指令推入队列
	QueuePush(uuid string, message interface{}) error
	// QueuePop 从队列中弹出指令
	QueuePop(uuid string) (interface{}, bool)
	// QueueIsEmpty 检查队列是否为空
	QueueIsEmpty(uuid string) bool

	// --- 查询 (Query) ---

	// ListDevices 分页列出设备，status 为 nil 时列出所有
	ListDevices(status *AuthenticateStatusType, page, size int) ([]DeviceRecord, error)
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

	IsEmpty(uuid string) bool
}
