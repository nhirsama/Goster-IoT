package device_manager

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

type DeviceManager struct {
	DataStore inter.DataStore

	// 缓存: Token -> UUID
	// 仅缓存状态为 Authenticated 的有效 Token
	tokenCache sync.Map

	// 运行时状态
	lastHeartbeat sync.Map // map[string]time.Time
	message       inter.MessageQueue
	DeathLine     time.Duration
}

func NewDeviceManager(ds inter.DataStore) inter.DeviceManager {
	return &DeviceManager{
		DataStore:     ds,
		tokenCache:    sync.Map{},
		lastHeartbeat: sync.Map{},
		message:       NewMessageQueue(100),
		DeathLine:     60 * time.Second, // 默认 60s 心跳超时
	}
}

// --- 身份与生命周期实现 ---

func (d *DeviceManager) GenerateUUID(meta inter.DeviceMetadata) (uuid string) {
	// 使用 SN + MAC 的 Hash 作为 UUID，保证同一设备生成 ID 固定
	sumSN := sha256.Sum256([]byte(meta.SerialNumber))
	sumMAC := sha256.Sum256([]byte(meta.MACAddress))

	combined := make([]byte, 64)
	copy(combined[:32], sumSN[:])
	copy(combined[32:], sumMAC[:])

	finalHash := sha256.Sum256(combined)
	uuid = hex.EncodeToString(finalHash[:])
	return uuid
}

// generateSecureToken 生成 32 字节随机 Token
func (d *DeviceManager) generateSecureToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// 备用方案 (极罕见情况)
		return d.generateLegacyToken("fallback")
	}
	return "gt_" + hex.EncodeToString(bytes)
}

func (d *DeviceManager) generateLegacyToken(seed string) string {
	// 仅作为随机数发生器失败时的兜底方案
	hash := sha256.Sum256([]byte(seed + time.Now().String()))
	return "gt_legacy_" + hex.EncodeToString(hash[:16])
}

func (d *DeviceManager) RegisterDevice(meta inter.DeviceMetadata) (err error) {
	uuid := d.GenerateUUID(meta)
	meta.AuthenticateStatus = inter.AuthenticatePending
	meta.Token = "" // 初始注册无 Token
	// datastore 会将空字符串处理为数据库 NULL
	return d.DataStore.InitDevice(uuid, meta)
}

func (d *DeviceManager) Authenticate(token string) (uuid string, err error) {
	// 1. 快速路径：检查缓存
	if val, ok := d.tokenCache.Load(token); ok {
		return val.(string), nil
	}

	// 2. 慢速路径：检查数据库
	uuid, status, err := d.DataStore.GetDeviceByToken(token)
	if err != nil {
		return "", inter.ErrInvalidToken
	}

	if status == inter.Authenticated {
		// 回填缓存
		d.tokenCache.Store(token, uuid)
		return uuid, nil
	}

	// 明确具体的拒绝原因
	switch status {
	case inter.AuthenticatePending:
		return uuid, inter.ErrDevicePending
	case inter.AuthenticateRefuse:
		return uuid, inter.ErrDeviceRefused
	default:
		return uuid, inter.ErrDeviceUnknown
	}
}

func (d *DeviceManager) UpdateDeviceAuthenticateStatus(uuid string, status inter.AuthenticateStatusType) (token string, err error) {
	meta, err := d.DataStore.LoadConfig(uuid)
	if err != nil {
		return "", err
	}

	// 如果存在旧 Token，从缓存中失效
	if meta.Token != "" {
		d.tokenCache.Delete(meta.Token)
	}

	meta.AuthenticateStatus = status

	switch status {
	case inter.Authenticated:
		// 生成新 Token
		meta.Token = d.generateSecureToken()
	case inter.AuthenticatePending, inter.AuthenticateRefuse, inter.AuthenticateRevoked:
		// 撤销 Token，设为空值
		meta.Token = ""
	default:

	}

	err = d.DataStore.SaveMetadata(uuid, meta)
	if err != nil {
		return "", err
	}

	return meta.Token, nil
}

func (d *DeviceManager) RefreshToken(uuid string) (newToken string, err error) {
	// 获取旧信息用于清理缓存
	meta, err := d.DataStore.LoadConfig(uuid)
	if err == nil && meta.Token != "" {
		d.tokenCache.Delete(meta.Token)
	}

	newToken = d.generateSecureToken()
	err = d.DataStore.UpdateToken(uuid, newToken)
	return newToken, err
}

func (d *DeviceManager) RevokeToken(uuid string) error {
	_, err := d.UpdateDeviceAuthenticateStatus(uuid, inter.AuthenticateRevoked)
	return err
}

// --- 管理端操作 ---

func (d *DeviceManager) ApproveDevice(uuid string) error {
	_, err := d.UpdateDeviceAuthenticateStatus(uuid, inter.Authenticated)
	return err
}

func (d *DeviceManager) RejectDevice(uuid string) error {
	_, err := d.UpdateDeviceAuthenticateStatus(uuid, inter.AuthenticateRefuse)
	return err
}

func (d *DeviceManager) UnblockDevice(uuid string) error {
	_, err := d.UpdateDeviceAuthenticateStatus(uuid, inter.AuthenticatePending)
	return err
}

func (d *DeviceManager) DeleteDevice(uuid string) error {
	// 先读出元数据以清理缓存
	meta, err := d.DataStore.LoadConfig(uuid)
	if err == nil && meta.Token != "" {
		d.tokenCache.Delete(meta.Token)
	}
	return d.DataStore.DestroyDevice(uuid)
}

func (d *DeviceManager) GetDeviceMetadata(uuid string) (inter.DeviceMetadata, error) {
	return d.DataStore.LoadConfig(uuid)
}

func (d *DeviceManager) ListDevices(status *inter.AuthenticateStatusType, page, size int) ([]inter.DeviceRecord, error) {
	if status == nil {
		return d.DataStore.ListDevices(page, size)
	}
	return d.DataStore.ListDevicesByStatus(*status, page, size)
}

// --- 运行时状态实现 ---

func (d *DeviceManager) HandleHeartbeat(uuid string) {
	d.lastHeartbeat.Store(uuid, time.Now())
}

func (d *DeviceManager) QueryDeviceStatus(uuid string) (inter.DeviceStatus, error) {
	if val, ok := d.lastHeartbeat.Load(uuid); ok {
		lastSeen := val.(time.Time)
		delta := time.Since(lastSeen)

		if delta < d.DeathLine {
			return inter.StatusOnline, nil
		}
		return inter.StatusOffline, nil
	}
	return inter.StatusOffline, errors.New("设备从未上线")
}

// --- 消息队列实现 ---

func (d *DeviceManager) QueuePush(uuid string, message interface{}) error {
	return d.message.Push(uuid, message)
}

func (d *DeviceManager) QueuePop(uuid string) (interface{}, bool) {
	return d.message.Pop(uuid)
}

func (d *DeviceManager) QueueIsEmpty(uuid string) bool {
	return d.message.IsEmpty(uuid)
}
