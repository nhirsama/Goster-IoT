package device_manager

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// DeviceRegistryService 负责设备身份、生命周期与管理端查询。
type DeviceRegistryService struct {
	dataStore  inter.DeviceRegistryStore
	hooks      DeviceRegistryHooks
	tokenCache sync.Map
}

// NewDeviceRegistry 创建设备身份与生命周期服务。
func NewDeviceRegistry(ds inter.DeviceRegistryStore) inter.DeviceRegistry {
	return NewDeviceRegistryWithHooks(ds, DeviceRegistryHooks{})
}

// NewDeviceRegistryWithHooks 创建设备身份服务，并允许注入生命周期副作用钩子。
func NewDeviceRegistryWithHooks(ds inter.DeviceRegistryStore, hooks DeviceRegistryHooks) inter.DeviceRegistry {
	return &DeviceRegistryService{
		dataStore: ds,
		hooks:     hooks,
	}
}

func (s *DeviceRegistryService) GenerateUUID(meta inter.DeviceMetadata) (uuid string) {
	sumSN := sha256.Sum256([]byte(meta.SerialNumber))
	sumMAC := sha256.Sum256([]byte(meta.MACAddress))

	combined := make([]byte, 64)
	copy(combined[:32], sumSN[:])
	copy(combined[32:], sumMAC[:])

	finalHash := sha256.Sum256(combined)
	uuid = hex.EncodeToString(finalHash[:])
	return uuid
}

func (s *DeviceRegistryService) RegisterDevice(meta inter.DeviceMetadata) error {
	uuid := s.GenerateUUID(meta)
	meta.AuthenticateStatus = inter.AuthenticatePending
	meta.Token = ""
	return s.dataStore.InitDevice(uuid, meta)
}

func (s *DeviceRegistryService) Authenticate(token string) (uuid string, err error) {
	if value, ok := s.tokenCache.Load(token); ok {
		return value.(string), nil
	}

	uuid, status, err := s.dataStore.GetDeviceByToken(token)
	if err != nil {
		return "", inter.ErrInvalidToken
	}
	if status == inter.Authenticated {
		s.tokenCache.Store(token, uuid)
		return uuid, nil
	}

	switch status {
	case inter.AuthenticatePending:
		return uuid, inter.ErrDevicePending
	case inter.AuthenticateRefuse:
		return uuid, inter.ErrDeviceRefused
	default:
		return uuid, inter.ErrDeviceUnknown
	}
}

func (s *DeviceRegistryService) UpdateDeviceAuthenticateStatus(uuid string, status inter.AuthenticateStatusType) (string, error) {
	meta, err := s.dataStore.LoadConfig(uuid)
	if err != nil {
		return "", err
	}
	if meta.Token != "" {
		s.tokenCache.Delete(meta.Token)
	}

	meta.AuthenticateStatus = status
	switch status {
	case inter.Authenticated:
		meta.Token = s.generateSecureToken()
	case inter.AuthenticatePending, inter.AuthenticateRefuse, inter.AuthenticateRevoked:
		meta.Token = ""
	}

	if err := s.dataStore.SaveMetadata(uuid, meta); err != nil {
		return "", err
	}
	return meta.Token, nil
}

func (s *DeviceRegistryService) RefreshToken(uuid string) (string, error) {
	meta, err := s.dataStore.LoadConfig(uuid)
	if err == nil && meta.Token != "" {
		s.tokenCache.Delete(meta.Token)
	}

	newToken := s.generateSecureToken()
	err = s.dataStore.UpdateToken(uuid, newToken)
	return newToken, err
}

func (s *DeviceRegistryService) RevokeToken(uuid string) error {
	_, err := s.UpdateDeviceAuthenticateStatus(uuid, inter.AuthenticateRevoked)
	return err
}

func (s *DeviceRegistryService) GetDeviceMetadata(uuid string) (inter.DeviceMetadata, error) {
	return s.dataStore.LoadConfig(uuid)
}

func (s *DeviceRegistryService) ApproveDevice(uuid string) error {
	_, err := s.UpdateDeviceAuthenticateStatus(uuid, inter.Authenticated)
	return err
}

func (s *DeviceRegistryService) RejectDevice(uuid string) error {
	_, err := s.UpdateDeviceAuthenticateStatus(uuid, inter.AuthenticateRefuse)
	return err
}

func (s *DeviceRegistryService) UnblockDevice(uuid string) error {
	_, err := s.UpdateDeviceAuthenticateStatus(uuid, inter.AuthenticatePending)
	return err
}

func (s *DeviceRegistryService) DeleteDevice(uuid string) error {
	meta, err := s.dataStore.LoadConfig(uuid)
	if err == nil && meta.Token != "" {
		s.tokenCache.Delete(meta.Token)
	}
	if err := s.dataStore.DestroyDevice(uuid); err != nil {
		return err
	}
	if s.hooks.OnDelete != nil {
		s.hooks.OnDelete(uuid)
	}
	return nil
}

func (s *DeviceRegistryService) ListDevices(status *inter.AuthenticateStatusType, page, size int) ([]inter.DeviceRecord, error) {
	if status == nil {
		return s.dataStore.ListDevices(page, size)
	}
	return s.dataStore.ListDevicesByStatus(*status, page, size)
}

func (s *DeviceRegistryService) ListDevicesByScope(scope inter.Scope, status *inter.AuthenticateStatusType, page, size int) ([]inter.DeviceRecord, error) {
	if strings.TrimSpace(scope.TenantID) == "" {
		return s.ListDevices(status, page, size)
	}
	return s.dataStore.ListDevicesByTenant(scope.TenantID, status, page, size)
}

func (s *DeviceRegistryService) GetDeviceMetadataByScope(scope inter.Scope, uuid string) (inter.DeviceMetadata, error) {
	if strings.TrimSpace(scope.TenantID) == "" {
		return s.GetDeviceMetadata(uuid)
	}
	return s.dataStore.LoadConfigByTenant(scope.TenantID, uuid)
}

func (s *DeviceRegistryService) generateSecureToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return s.generateLegacyToken("fallback")
	}
	return "gt_" + hex.EncodeToString(bytes)
}

func (s *DeviceRegistryService) generateLegacyToken(seed string) string {
	hash := sha256.Sum256([]byte(seed + time.Now().String()))
	return "gt_legacy_" + hex.EncodeToString(hash[:16])
}
