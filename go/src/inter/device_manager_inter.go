package inter

import (
	"errors"
	"time"
)

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

// DeviceRegistry 定义设备身份、生命周期与管理端查询能力。
type DeviceRegistry interface {
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

	// ListDevices 分页列出设备，status 为 nil 时列出所有
	ListDevices(status *AuthenticateStatusType, page, size int) ([]DeviceRecord, error)

	// ListDevicesByScope 在给定授权范围内列出设备。
	ListDevicesByScope(scope Scope, status *AuthenticateStatusType, page, size int) ([]DeviceRecord, error)

	// GetDeviceMetadataByScope 在给定授权范围内查询设备详情。
	GetDeviceMetadataByScope(scope Scope, uuid string) (DeviceMetadata, error)
}

// DevicePresence 定义设备在线状态能力。
type DevicePresence interface {
	// HandleHeartbeat 处理设备心跳
	HandleHeartbeat(uuid string)

	// QueryDeviceStatus 查询设备在线状态
	QueryDeviceStatus(uuid string) (DeviceStatus, error)
}

// DeviceCommandQueue 定义面向设备的下行命令缓冲能力。
// 当前默认实现仍在内存中，后续可替换为 Redis 等共享队列。
type DeviceCommandQueue interface {
	// Enqueue 将下行消息推入设备队列。
	Enqueue(uuid string, message DownlinkMessage) error
	// Dequeue 从设备队列中弹出最早的一条下行消息。
	Dequeue(uuid string) (DownlinkMessage, bool, error)
	// IsEmpty 检查设备队列是否为空。
	IsEmpty(uuid string) bool
}

// DownlinkCommandService 定义下行命令的编排能力。
// 它负责命令持久化、入队以及发送状态流转，避免由 Web 或 Gateway 直接操作底层队列和命令表。
type DownlinkCommandService interface {
	Enqueue(scope Scope, uuid string, cmdID CmdID, command string, payloadJSON []byte) (DownlinkMessage, error)
	PopDownlink(uuid string) (DownlinkMessage, bool, error)
	MarkSent(commandID int64) error
	MarkAcked(commandID int64) error
	MarkFailed(commandID int64, errorText string) error
}

// ExternalEntityService 定义外部集成实体的管理能力。
type ExternalEntityService interface {
	// GenerateExternalUUID 为外部实体生成稳定 UUID
	GenerateExternalUUID(source, entityID string) string

	// UpsertExternalEntity 创建或更新外部实体主档
	UpsertExternalEntity(entity ExternalEntity) error

	// ListExternalEntities 按 source/domain 分页列出外部实体
	ListExternalEntities(source, domain string, page, size int) ([]ExternalEntity, error)

	// BatchAppendExternalObservations 批量写入外部观测值
	BatchAppendExternalObservations(items []ExternalObservation) error

	// QueryExternalObservations 查询外部观测值
	QueryExternalObservations(source, entityID string, start, end int64, limit int) ([]ExternalObservation, error)
}

// TelemetryIngestService 定义设备遥测数据的接收与落库能力。
// 网络层只负责协议与会话，这里的服务负责把解析后的数据沉淀到核心系统。
type TelemetryIngestService interface {
	IngestMetrics(uuid string, points []MetricPoint) error
	IngestLog(uuid string, data LogUploadData) error
	IngestEvent(uuid string, payload []byte) error
	IngestDeviceError(uuid string, payload []byte) error
}

// DeviceManager 是当前阶段保留的组合接口。
// 新调用方应优先依赖更小的 DeviceRegistry / DevicePresence / ExternalEntityService。
type DeviceManager interface {
	DeviceRegistry
	DevicePresence
	ExternalEntityService
}

// DevicePresenceStore 抽象设备在线状态的运行时存储。
// 当前默认实现仍在内存中，后续可替换为 Redis 等共享存储。
type DevicePresenceStore interface {
	SaveLastSeen(uuid string, at time.Time)
	LoadLastSeen(uuid string) (time.Time, bool)
	Delete(uuid string)
}
