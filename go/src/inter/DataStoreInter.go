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

	// [用户管理]

	// RegisterUser 注册一个新用户。
	// username 为用户名（唯一），password 为明文密码，permission 为权限字段。
	RegisterUser(username, password string, permission PermissionType) error

	// LoginUser 用户登录验证。
	// 验证成功返回用户的权限级别，失败返回错误。
	LoginUser(username, password string) (PermissionType, error)

	// ChangePassword 修改用户密码。
	// 需要验证 oldPassword 是否正确，如果正确则更新为 newPassword。
	ChangePassword(username, oldPassword, newPassword string) error

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
