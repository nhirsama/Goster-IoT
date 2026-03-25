package device_manager

import (
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

type DeviceManager struct {
	registry inter.DeviceRegistry
	presence *DevicePresenceService
	message  inter.MessageQueue
	external inter.ExternalEntityService
}

func NewDeviceManager(ds inter.DataStore) inter.DeviceManager {
	return NewDeviceManagerWithConfig(ds, appcfg.DefaultDeviceManagerConfig())
}

func NewDeviceManagerWithConfig(ds inter.DataStore, cfg appcfg.DeviceManagerConfig) inter.DeviceManager {
	n := appcfg.NormalizeDeviceManagerConfig(cfg)
	return &DeviceManager{
		registry: NewDeviceRegistry(ds),
		presence: NewDevicePresenceWithStore(n.HeartbeatDeadline, NewInMemoryDevicePresenceStore()),
		message:  NewMessageQueue(n.QueueCapacity),
		external: NewExternalEntityService(ds, n),
	}
}

// SetHeartbeatDeadline 允许测试或装配层调整在线判定阈值。
func (d *DeviceManager) SetHeartbeatDeadline(deadline time.Duration) {
	if d.presence != nil {
		d.presence.SetDeadline(deadline)
	}
}

// --- DeviceRegistry 过渡转发 ---

func (d *DeviceManager) GenerateUUID(meta inter.DeviceMetadata) string {
	return d.registry.GenerateUUID(meta)
}

func (d *DeviceManager) RegisterDevice(meta inter.DeviceMetadata) error {
	return d.registry.RegisterDevice(meta)
}

func (d *DeviceManager) Authenticate(token string) (string, error) {
	return d.registry.Authenticate(token)
}

func (d *DeviceManager) UpdateDeviceAuthenticateStatus(uuid string, status inter.AuthenticateStatusType) (string, error) {
	return d.registry.UpdateDeviceAuthenticateStatus(uuid, status)
}

func (d *DeviceManager) RefreshToken(uuid string) (string, error) {
	return d.registry.RefreshToken(uuid)
}

func (d *DeviceManager) RevokeToken(uuid string) error {
	return d.registry.RevokeToken(uuid)
}

func (d *DeviceManager) GetDeviceMetadata(uuid string) (inter.DeviceMetadata, error) {
	return d.registry.GetDeviceMetadata(uuid)
}

func (d *DeviceManager) ApproveDevice(uuid string) error {
	return d.registry.ApproveDevice(uuid)
}

func (d *DeviceManager) RejectDevice(uuid string) error {
	return d.registry.RejectDevice(uuid)
}

func (d *DeviceManager) UnblockDevice(uuid string) error {
	return d.registry.UnblockDevice(uuid)
}

func (d *DeviceManager) DeleteDevice(uuid string) error {
	if err := d.registry.DeleteDevice(uuid); err != nil {
		return err
	}
	if d.presence != nil {
		d.presence.delete(uuid)
	}
	return nil
}

func (d *DeviceManager) ListDevices(status *inter.AuthenticateStatusType, page, size int) ([]inter.DeviceRecord, error) {
	return d.registry.ListDevices(status, page, size)
}

func (d *DeviceManager) ListDevicesByScope(scope inter.Scope, status *inter.AuthenticateStatusType, page, size int) ([]inter.DeviceRecord, error) {
	return d.registry.ListDevicesByScope(scope, status, page, size)
}

func (d *DeviceManager) GetDeviceMetadataByScope(scope inter.Scope, uuid string) (inter.DeviceMetadata, error) {
	return d.registry.GetDeviceMetadataByScope(scope, uuid)
}

func (d *DeviceManager) GenerateExternalUUID(source, entityID string) string {
	return d.external.GenerateExternalUUID(source, entityID)
}

func (d *DeviceManager) UpsertExternalEntity(entity inter.ExternalEntity) error {
	return d.external.UpsertExternalEntity(entity)
}

func (d *DeviceManager) ListExternalEntities(source, domain string, page, size int) ([]inter.ExternalEntity, error) {
	return d.external.ListExternalEntities(source, domain, page, size)
}

func (d *DeviceManager) BatchAppendExternalObservations(items []inter.ExternalObservation) error {
	return d.external.BatchAppendExternalObservations(items)
}

func (d *DeviceManager) QueryExternalObservations(source, entityID string, start, end int64, limit int) ([]inter.ExternalObservation, error) {
	return d.external.QueryExternalObservations(source, entityID, start, end, limit)
}

func (d *DeviceManager) HandleHeartbeat(uuid string) {
	d.presence.HandleHeartbeat(uuid)
}

func (d *DeviceManager) QueryDeviceStatus(uuid string) (inter.DeviceStatus, error) {
	return d.presence.QueryDeviceStatus(uuid)
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
