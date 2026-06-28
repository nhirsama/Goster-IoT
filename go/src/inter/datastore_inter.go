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

// DeviceRepository 描述设备主档、令牌与生命周期相关的持久化能力。
type DeviceRepository interface {
	// InitDevice 初始化一个新的设备存储空间。
	InitDevice(uuid string, meta DeviceMetadata) error

	// DestroyDevice 彻底删除指定设备的所有数据。
	DestroyDevice(uuid string) error

	// LoadConfig 从存储中读取指定设备的配置信息。
	LoadConfig(uuid string) (out DeviceMetadata, err error)

	// SaveMetadata 将元信息持久化到存储中。
	SaveMetadata(uuid string, meta DeviceMetadata) error

	// ListDevices 分页查询已注册的设备列表。
	ListDevices(page, size int) ([]DeviceRecord, error)

	// ListDevicesByStatus 根据认证状态分页查询设备列表。
	ListDevicesByStatus(status AuthenticateStatusType, page, size int) ([]DeviceRecord, error)

	// GetDeviceByToken 根据 Token 查找对应的设备 UUID。
	GetDeviceByToken(token string) (uuid string, Status AuthenticateStatusType, err error)

	// UpdateToken 更新指定设备的 Token。
	UpdateToken(uuid string, newToken string) error
}

// ScopedDeviceRepository 描述带租户范围约束的设备查询能力。
type ScopedDeviceRepository interface {
	// ResolveDeviceTenant 查询设备所属租户。
	ResolveDeviceTenant(uuid string) (tenantID string, err error)

	// LoadConfigByTenant 在指定租户范围内读取设备配置。
	LoadConfigByTenant(tenantID, uuid string) (out DeviceMetadata, err error)

	// ListDevicesByTenant 在指定租户范围内分页列出设备。
	ListDevicesByTenant(tenantID string, status *AuthenticateStatusType, page, size int) ([]DeviceRecord, error)
}

// MetricsRepository 描述设备时序指标的持久化能力。
type MetricsRepository interface {
	AppendMetric(uuid string, points MetricPoint) error
	BatchAppendMetrics(uuid string, points []MetricPoint) error
	QueryMetrics(uuid string, start, end int64) ([]MetricPoint, error)
	QueryMetricsByTenant(tenantID, uuid string, start, end int64) ([]MetricPoint, error)
}

// DeviceLogRepository 描述设备日志的持久化能力。
type DeviceLogRepository interface {
	WriteLog(uuid string, level string, message string) error
}

// DeviceCommandRepository 描述设备下行指令日志的持久化能力。
type DeviceCommandRepository interface {
	CreateDeviceCommand(uuid string, cmdID CmdID, command string, payloadJSON []byte) (int64, error)
	CreateDeviceCommandByTenant(tenantID, uuid string, cmdID CmdID, command string, payloadJSON []byte) (int64, error)
	UpdateDeviceCommandStatus(commandID int64, status DeviceCommandStatus, errorText string) error
}

// ExternalEntityRepository 描述外部集成实体与观测值的持久化能力。
type ExternalEntityRepository interface {
	UpsertExternalEntity(entity ExternalEntity) error
	GetExternalEntity(source, entityID string) (ExternalEntity, error)
	ListExternalEntities(source, domain string, limit, offset int) ([]ExternalEntity, error)
	BatchAppendExternalObservations(items []ExternalObservation) error
	QueryExternalObservations(source, entityID string, start, end int64, limit int) ([]ExternalObservation, error)
}

// UserRepository 描述平台用户与权限的持久化能力。
type UserRepository interface {
	GetUserCount() (int, error)
	ListUsers() ([]User, error)
	GetUserPermission(username string) (PermissionType, error)
	UpdateUserPermission(username string, perm PermissionType) error
}

// TenantRoleRepository 描述用户与租户角色关系的查询能力。
type TenantRoleRepository interface {
	GetUserTenantRoles(username string) (map[string]TenantRole, error)
}

// DeviceRegistryStore 是设备注册服务依赖的最小仓储组合。
type DeviceRegistryStore interface {
	DeviceRepository
	ScopedDeviceRepository
}

// TelemetryStore 是遥测接收服务依赖的最小仓储组合。
type TelemetryStore interface {
	MetricsRepository
	DeviceLogRepository
}

// CoreStore 是核心业务装配依赖的最小仓储组合。
type CoreStore interface {
	DeviceRegistryStore
	TelemetryStore
	DeviceCommandRepository
	ExternalEntityRepository
}

// WebV1Store 是当前 v1 HTTP 接口依赖的最小仓储组合。
type WebV1Store interface {
	MetricsRepository
	UserRepository
	TenantRoleRepository
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
