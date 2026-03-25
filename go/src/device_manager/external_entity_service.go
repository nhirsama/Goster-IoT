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

// ExternalEntityService 负责外部实体与观测值相关业务。
type ExternalEntityService struct {
	dataStore               inter.DataStore
	listDefaultSize         int
	listMaxSize             int
	observationDefaultLimit int
	observationMaxLimit     int
}

// NewExternalEntityService 创建外部实体服务。
func NewExternalEntityService(ds inter.DataStore, cfg appcfg.DeviceManagerConfig) inter.ExternalEntityService {
	n := appcfg.NormalizeDeviceManagerConfig(cfg)
	return &ExternalEntityService{
		dataStore:               ds,
		listDefaultSize:         n.ExternalListPage.DefaultSize,
		listMaxSize:             n.ExternalListPage.MaxSize,
		observationDefaultLimit: n.ExternalObservationLimit.Default,
		observationMaxLimit:     n.ExternalObservationLimit.Max,
	}
}

func (s *ExternalEntityService) GenerateExternalUUID(source, entityID string) string {
	raw := strings.ToLower(strings.TrimSpace(source)) + ":" + strings.TrimSpace(entityID)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *ExternalEntityService) UpsertExternalEntity(entity inter.ExternalEntity) error {
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
		entity.GosterUUID = s.GenerateExternalUUID(entity.Source, entity.EntityID)
	}
	if entity.LastStateTS <= 0 {
		entity.LastStateTS = time.Now().UnixMilli()
	}
	return s.dataStore.UpsertExternalEntity(entity)
}

func (s *ExternalEntityService) ListExternalEntities(source, domain string, page, size int) ([]inter.ExternalEntity, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = s.listDefaultSize
	}
	if size > s.listMaxSize {
		size = s.listMaxSize
	}
	offset := (page - 1) * size
	return s.dataStore.ListExternalEntities(source, domain, size, offset)
}

func (s *ExternalEntityService) BatchAppendExternalObservations(items []inter.ExternalObservation) error {
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
			item.ValueSig = s.buildExternalObservationSignature(item)
		}
		normalized = append(normalized, item)
	}
	return s.dataStore.BatchAppendExternalObservations(normalized)
}

func (s *ExternalEntityService) QueryExternalObservations(source, entityID string, start, end int64, limit int) ([]inter.ExternalObservation, error) {
	source = strings.TrimSpace(source)
	entityID = strings.TrimSpace(entityID)
	if source == "" || entityID == "" {
		return nil, errors.New("source/entity_id is required")
	}
	if limit <= 0 {
		limit = s.observationDefaultLimit
	}
	if limit > s.observationMaxLimit {
		limit = s.observationMaxLimit
	}
	return s.dataStore.QueryExternalObservations(source, entityID, start, end, limit)
}

func (s *ExternalEntityService) buildExternalObservationSignature(item inter.ExternalObservation) string {
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
