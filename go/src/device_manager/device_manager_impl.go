package device_manager

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

type DeviceManager struct {
	DataStore inter.DataStore

	registry inter.DeviceRegistry
	presence *DevicePresenceService
	message  inter.MessageQueue

	externalListDefaultSize int
	externalListMaxSize     int
	externalObsDefaultLimit int
	externalObsMaxLimit     int
}

func NewDeviceManager(ds inter.DataStore) inter.DeviceManager {
	return NewDeviceManagerWithConfig(ds, appcfg.DefaultDeviceManagerConfig())
}

func NewDeviceManagerWithConfig(ds inter.DataStore, cfg appcfg.DeviceManagerConfig) inter.DeviceManager {
	n := appcfg.NormalizeDeviceManagerConfig(cfg)
	return &DeviceManager{
		DataStore: ds,
		registry:  NewDeviceRegistry(ds),
		presence:  NewDevicePresenceWithStore(n.HeartbeatDeadline, NewInMemoryDevicePresenceStore()),
		message:   NewMessageQueue(n.QueueCapacity),

		externalListDefaultSize: n.ExternalListPage.DefaultSize,
		externalListMaxSize:     n.ExternalListPage.MaxSize,
		externalObsDefaultLimit: n.ExternalObservationLimit.Default,
		externalObsMaxLimit:     n.ExternalObservationLimit.Max,
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
	raw := strings.ToLower(strings.TrimSpace(source)) + ":" + strings.TrimSpace(entityID)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (d *DeviceManager) UpsertExternalEntity(entity inter.ExternalEntity) error {
	entity.Source = strings.TrimSpace(entity.Source)
	entity.EntityID = strings.TrimSpace(entity.EntityID)
	entity.Domain = strings.TrimSpace(entity.Domain)
	entity.ValueType = strings.TrimSpace(entity.ValueType)
	if entity.Source == "" || entity.EntityID == "" || entity.Domain == "" {
		return errors.New("source/entity_id/domain is required")
	}
	if entity.ValueType == "" {
		entity.ValueType = "string"
	}
	if entity.GosterUUID == "" {
		entity.GosterUUID = d.GenerateExternalUUID(entity.Source, entity.EntityID)
	}
	if entity.LastStateTS <= 0 {
		entity.LastStateTS = time.Now().UnixMilli()
	}
	return d.DataStore.UpsertExternalEntity(entity)
}

func (d *DeviceManager) ListExternalEntities(source, domain string, page, size int) ([]inter.ExternalEntity, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = d.externalListDefaultSize
	}
	if size > d.externalListMaxSize {
		size = d.externalListMaxSize
	}
	offset := (page - 1) * size
	return d.DataStore.ListExternalEntities(source, domain, size, offset)
}

func (d *DeviceManager) BatchAppendExternalObservations(items []inter.ExternalObservation) error {
	if len(items) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	normalized := make([]inter.ExternalObservation, 0, len(items))
	for _, item := range items {
		item.Source = strings.TrimSpace(item.Source)
		item.EntityID = strings.TrimSpace(item.EntityID)
		if item.Source == "" || item.EntityID == "" {
			return errors.New("source/entity_id is required")
		}
		if item.Timestamp <= 0 {
			item.Timestamp = now
		}
		if strings.TrimSpace(item.ValueSig) == "" {
			item.ValueSig = d.buildExternalObservationSignature(item)
		}
		normalized = append(normalized, item)
	}
	return d.DataStore.BatchAppendExternalObservations(normalized)
}

func (d *DeviceManager) QueryExternalObservations(source, entityID string, start, end int64, limit int) ([]inter.ExternalObservation, error) {
	source = strings.TrimSpace(source)
	entityID = strings.TrimSpace(entityID)
	if source == "" || entityID == "" {
		return nil, errors.New("source/entity_id is required")
	}
	if limit <= 0 {
		limit = d.externalObsDefaultLimit
	}
	if limit > d.externalObsMaxLimit {
		limit = d.externalObsMaxLimit
	}
	return d.DataStore.QueryExternalObservations(source, entityID, start, end, limit)
}

func (d *DeviceManager) HandleHeartbeat(uuid string) {
	d.presence.HandleHeartbeat(uuid)
}

func (d *DeviceManager) QueryDeviceStatus(uuid string) (inter.DeviceStatus, error) {
	return d.presence.QueryDeviceStatus(uuid)
}

func (d *DeviceManager) buildExternalObservationSignature(item inter.ExternalObservation) string {
	var payload string
	switch {
	case item.ValueNum != nil:
		payload = fmt.Sprintf("n:%g|u:%s", *item.ValueNum, item.Unit)
	case item.ValueBool != nil:
		payload = fmt.Sprintf("b:%t|u:%s", *item.ValueBool, item.Unit)
	case item.ValueText != nil:
		payload = fmt.Sprintf("t:%s|u:%s", *item.ValueText, item.Unit)
	case len(item.ValueJSON) > 0:
		b, _ := json.Marshal(item.ValueJSON)
		payload = fmt.Sprintf("j:%s|u:%s", string(b), item.Unit)
	default:
		payload = fmt.Sprintf("empty|u:%s", item.Unit)
	}
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:8])
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
