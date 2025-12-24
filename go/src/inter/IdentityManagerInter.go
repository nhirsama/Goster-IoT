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

// IdentityManager 定义了设备身份认证与安全管理的标准接口。
// 它负责将外部凭证（Token）转化为系统内部标识（UUID），并管理设备的准入生命周期。
type IdentityManager interface {
	// [身份生成]

	// GenerateUUID 根据设备唯一的硬件标识符（如芯片 uid）生成系统唯一的 UUID。
	// 该过程应是确定性的，即相同的 DeviceMetadata.SerialNumber 和 DeviceMetadata.MACAddress 始终生成相同的 UUID。
	GenerateUUID(meta DeviceMetadata) (uuid string)

	// [身份注册与签发]

	// RegisterDevice 注册一个新设备到系统中。
	// hwID: 硬件原始 ID；name: 设备别名。
	// 返回生成的 UUID 和访问令牌 Token。
	RegisterDevice(uuid string, meta DeviceMetadata) (token string, err error)
	// [凭证校验]

	// Authenticate 验证传入 Token 的合法性。
	// 如果验证通过，返回该设备对应的 UUID，否则返回 ErrInvalidToken。
	// 这是一个高频调用接口，实现层应利用 DataStore 的内存索引进行加速。
	Authenticate(token string) (uuid string, err error)

	// [凭证管理]

	// RefreshToken 为指定设备重新生成访问令牌。
	// 旧的 Token 将失效，新的 Token 会同步到存储模块中。
	RefreshToken(uuid string) (newToken string, err error)

	// RevokeToken 吊销指定设备的访问权限。
	// 该操作会清除存储中的 Token，使设备立即失去访问资格。
	RevokeToken(uuid string) error
}
