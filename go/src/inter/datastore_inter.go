package inter

import "time"

type AuthenticateStatusType int

const (
	Authenticated       AuthenticateStatusType = iota // 已认证
	AuthenticateRefuse                                // 拒绝认证
	AuthenticatePending                               // 等待认证
	AuthenticateUnknown                               // 未知的设备
	AuthenticateRevoked                               // 已吊销

)

// PermissionType 用户权限类型
type PermissionType int

const (
	PermissionNone      PermissionType = iota // 零权限
	PermissionReadOnly                        // 只读
	PermissionReadWrite                       // 读写
	PermissionAdmin                           // 管理员
)

// DeviceCommandStatus 设备下行指令状态
type DeviceCommandStatus string

const (
	DeviceCommandStatusQueued DeviceCommandStatus = "queued"
	DeviceCommandStatusSent   DeviceCommandStatus = "sent"
	DeviceCommandStatusAcked  DeviceCommandStatus = "acked"
	DeviceCommandStatusFailed DeviceCommandStatus = "failed"
)

// DeviceMetadata 设备静态元数据
type DeviceMetadata struct {
	Name               string                 `json:"name"`               // 设备名称
	HWVersion          string                 `json:"hw_version"`         // 硬件版本
	SWVersion          string                 `json:"sw_version"`         // 固件/软件版本
	ConfigVersion      string                 `json:"config_version"`     // 配置文件版本
	SerialNumber       string                 `json:"sn"`                 // 序列号
	MACAddress         string                 `json:"mac"`                // Mac 地址
	CreatedAt          time.Time              `json:"created_at"`         // 首次注册时间
	Token              string                 `json:"token"`              // 设备 Token
	AuthenticateStatus AuthenticateStatusType `json:"authenticateStatus"` // 设备认证状态
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
	Type      uint8   `json:"type"`  // 数据类型 (1=Temp, 2=Humi, 4=Lux)
}

// ExternalEntity 外部集成平台实体（如 Home Assistant 中的 entity）
type ExternalEntity struct {
	Source      string                 `json:"source"`
	EntityID    string                 `json:"entity_id"`
	Domain      string                 `json:"domain"`
	GosterUUID  string                 `json:"goster_uuid,omitempty"`
	DeviceID    string                 `json:"device_id,omitempty"`
	Model       string                 `json:"model,omitempty"`
	Name        string                 `json:"name,omitempty"`
	RoomName    string                 `json:"room_name,omitempty"`
	Unit        string                 `json:"unit,omitempty"`
	ValueType   string                 `json:"value_type"` // number | bool | string | json
	DeviceClass string                 `json:"device_class,omitempty"`
	StateClass  string                 `json:"state_class,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
	LastStateTS int64                  `json:"last_state_ts,omitempty"`
	LastText    *string                `json:"last_state_text,omitempty"`
	LastNum     *float64               `json:"last_state_num,omitempty"`
	LastBool    *bool                  `json:"last_state_bool,omitempty"`
}

// ExternalObservation 外部实体观测值（通用多类型）
type ExternalObservation struct {
	Source    string                 `json:"source"`
	EntityID  string                 `json:"entity_id"`
	Timestamp int64                  `json:"ts"`
	ValueNum  *float64               `json:"value_num,omitempty"`
	ValueText *string                `json:"value_text,omitempty"`
	ValueBool *bool                  `json:"value_bool,omitempty"`
	ValueJSON map[string]interface{} `json:"value_json,omitempty"`
	Unit      string                 `json:"unit,omitempty"`
	ValueSig  string                 `json:"value_sig,omitempty"` // 幂等签名
	RawEvent  map[string]interface{} `json:"raw_event,omitempty"`
}

// DataStore 定义了底层数据持久化的标准接口，用于管理设备生命周期、配置、时序指标及日志。
// 该接口旨在兼容多种存储后端（如 SQLite, PostgreSQL 或时序数据库）。
type DataStore interface {
	// [生命周期管理]

	// InitDevice 初始化一个新的设备存储空间。
	// uuid 是设备的唯一标识符，meta 包含设备的初始元数据（如型号、硬件版本等）。
	// 如果设备已存在，应返回错误。
	InitDevice(uuid string, meta DeviceMetadata) error

	// DestroyDevice 彻底删除指定设备的所有数据，包括配置、时序指标和日志。
	DestroyDevice(uuid string) error

	// [配置与元数据管理]

	// LoadConfig 从存储中读取指定设备的配置信息。
	LoadConfig(uuid string) (out DeviceMetadata, err error)

	// SaveMetadata 将元信息持久化到存储中（冷数据存储）。
	// meta 是要保存的配置对象，该方法会覆盖原有的配置。
	SaveMetadata(uuid string, meta DeviceMetadata) error

	// ListDevices 分页查询已注册的设备列表。
	// page 指定页码（通常从 1 开始），size 指定每页返回的条数。
	ListDevices(page, size int) ([]DeviceRecord, error)

	// ListDevicesByStatus 根据认证状态分页查询设备列表
	ListDevicesByStatus(status AuthenticateStatusType, page, size int) ([]DeviceRecord, error)

	// [时序数据管理]

	// AppendMetric 向指定设备追加一条时序数据。
	// ts 为 Unix 时间戳（秒或毫秒，取决于系统实现），value 为传感器采集的浮点数值。
	AppendMetric(uuid string, points MetricPoint) error

	// BatchAppendMetrics 同时插入多条数据
	BatchAppendMetrics(uuid string, points []MetricPoint) error

	// QueryMetrics 查询指定时间范围内的时序数据。
	// start 和 end 分别为开始和结束的时间戳（闭区间）。
	QueryMetrics(uuid string, start, end int64) ([]MetricPoint, error)

	// [外部集成实体与观测]

	// UpsertExternalEntity 创建或更新外部实体主档
	UpsertExternalEntity(entity ExternalEntity) error

	// GetExternalEntity 查询单个外部实体
	GetExternalEntity(source, entityID string) (ExternalEntity, error)

	// ListExternalEntities 按 source/domain 分页查询外部实体
	ListExternalEntities(source, domain string, limit, offset int) ([]ExternalEntity, error)

	// BatchAppendExternalObservations 批量追加外部观测值（支持去重）
	BatchAppendExternalObservations(items []ExternalObservation) error

	// QueryExternalObservations 查询外部实体时序观测值
	QueryExternalObservations(source, entityID string, start, end int64, limit int) ([]ExternalObservation, error)

	// [设备指令日志]

	// CreateDeviceCommand 创建一条设备下行指令日志，返回指令 ID。
	CreateDeviceCommand(uuid string, cmdID CmdID, command string, payloadJSON []byte) (int64, error)

	// UpdateDeviceCommandStatus 更新设备下行指令状态。
	UpdateDeviceCommandStatus(commandID int64, status DeviceCommandStatus, errorText string) error

	// [日志管理]

	// WriteLog 记录一条与设备相关的运行日志。
	// level 通常为 "info", "warn", "error" 等级别，用于后续过滤。
	WriteLog(uuid string, level string, message string) error

	// [权限与映射管理]

	// GetDeviceByToken 根据 Token 查找对应的设备 UUID。
	GetDeviceByToken(token string) (uuid string, Status AuthenticateStatusType, err error)

	// UpdateToken 更新指定设备的 Token。
	// 用于 Token 过期重刷或安全性重置场景。
	UpdateToken(uuid string, newToken string) error

	// [多租户范围查询]

	// ResolveDeviceTenant 查询设备所属租户。
	ResolveDeviceTenant(uuid string) (tenantID string, err error)

	// LoadConfigByTenant 在指定租户范围内读取设备配置。
	LoadConfigByTenant(tenantID, uuid string) (out DeviceMetadata, err error)

	// ListDevicesByTenant 在指定租户范围内分页列出设备。
	ListDevicesByTenant(tenantID string, status *AuthenticateStatusType, page, size int) ([]DeviceRecord, error)

	// QueryMetricsByTenant 在指定租户范围内查询设备指标。
	QueryMetricsByTenant(tenantID, uuid string, start, end int64) ([]MetricPoint, error)

	// CreateDeviceCommandByTenant 在指定租户范围内创建下行指令日志。
	CreateDeviceCommandByTenant(tenantID, uuid string, cmdID CmdID, command string, payloadJSON []byte) (int64, error)

	// [用户管理]

	// GetUserCount 获取注册用户总数
	GetUserCount() (int, error)

	// ListUsers 获取所有用户列表（仅管理员可用）
	ListUsers() ([]User, error)

	// GetUserPermission 获取指定用户的当前权限
	GetUserPermission(username string) (PermissionType, error)

	// UpdateUserPermission 更新用户权限（仅管理员可用）
	UpdateUserPermission(username string, perm PermissionType) error
}

// User 用户信息
type User struct {
	Username   string         `json:"username"`
	Permission PermissionType `json:"permission"`
	CreatedAt  time.Time      `json:"created_at"`
}

// SessionUser 定义了 web 层访问当前登录用户所需的接口
// 解耦 web 层与 DataStore 的具体实现
type SessionUser interface {
	GetPID() string
	GetEmail() string
	GetUsername() string
	GetPermission() PermissionType
}
