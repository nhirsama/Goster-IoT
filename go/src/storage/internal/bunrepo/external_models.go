package bunrepo

import (
	"database/sql"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/uptrace/bun"
)

type DeviceCommandRow struct {
	bun.BaseModel `bun:"table:integration_external_commands"`

	ID          int64      `bun:"id,pk,autoincrement"`
	TenantID    string     `bun:"tenant_id"`
	Source      string     `bun:"source"`
	EntityID    string     `bun:"entity_id"`
	Command     string     `bun:"command"`
	PayloadJSON *string    `bun:"payload_json"`
	Status      string     `bun:"status"`
	ErrorText   *string    `bun:"error_text"`
	RequestedAt time.Time  `bun:"requested_at"`
	ExecutedAt  *time.Time `bun:"executed_at"`
}

type ExternalEntityRow struct {
	bun.BaseModel `bun:"table:integration_external_entities"`

	ID             int64           `bun:"id,pk,autoincrement"`
	TenantID       string          `bun:"tenant_id"`
	Source         string          `bun:"source"`
	EntityID       string          `bun:"entity_id"`
	Domain         string          `bun:"domain"`
	GosterUUID     string          `bun:"goster_uuid"`
	DeviceID       string          `bun:"device_id"`
	Model          string          `bun:"model"`
	Name           string          `bun:"name"`
	RoomName       string          `bun:"room_name"`
	Unit           string          `bun:"unit"`
	ValueType      string          `bun:"value_type"`
	DeviceClass    string          `bun:"device_class"`
	StateClass     string          `bun:"state_class"`
	AttributesJSON sql.NullString  `bun:"attributes_json"`
	LastStateText  sql.NullString  `bun:"last_state_text"`
	LastStateNum   sql.NullFloat64 `bun:"last_state_num"`
	LastStateBool  sql.NullInt64   `bun:"last_state_bool"`
	LastSeenTS     int64           `bun:"last_seen_ts"`
}

type ExternalObservationRow struct {
	bun.BaseModel `bun:"table:integration_external_observations"`

	ID           int64           `bun:"id,pk,autoincrement"`
	TenantID     string          `bun:"tenant_id"`
	Source       string          `bun:"source"`
	EntityID     string          `bun:"entity_id"`
	TS           int64           `bun:"ts"`
	ValueNum     sql.NullFloat64 `bun:"value_num"`
	ValueText    sql.NullString  `bun:"value_text"`
	ValueBool    sql.NullInt64   `bun:"value_bool"`
	ValueJSON    sql.NullString  `bun:"value_json"`
	Unit         string          `bun:"unit"`
	ValueSig     string          `bun:"value_sig"`
	RawEventJSON sql.NullString  `bun:"raw_event_json"`
}

func NewExternalEntityRow(entity inter.ExternalEntity) (*ExternalEntityRow, error) {
	attrsJSON, err := NullableJSONString(entity.Attributes)
	if err != nil {
		return nil, err
	}
	return &ExternalEntityRow{
		TenantID:       DefaultTenantID,
		Source:         strings.TrimSpace(entity.Source),
		EntityID:       strings.TrimSpace(entity.EntityID),
		Domain:         strings.TrimSpace(entity.Domain),
		GosterUUID:     strings.TrimSpace(entity.GosterUUID),
		DeviceID:       strings.TrimSpace(entity.DeviceID),
		Model:          strings.TrimSpace(entity.Model),
		Name:           strings.TrimSpace(entity.Name),
		RoomName:       strings.TrimSpace(entity.RoomName),
		Unit:           strings.TrimSpace(entity.Unit),
		ValueType:      strings.TrimSpace(entity.ValueType),
		DeviceClass:    strings.TrimSpace(entity.DeviceClass),
		StateClass:     strings.TrimSpace(entity.StateClass),
		AttributesJSON: attrsJSON,
		LastStateText:  NullableStringPtr(entity.LastText),
		LastStateNum:   NullableFloat64(entity.LastNum),
		LastStateBool:  BoolPtrToNullableInt(entity.LastBool),
		LastSeenTS:     entity.LastStateTS,
	}, nil
}

func (r ExternalEntityRow) ToExternalEntity() (inter.ExternalEntity, error) {
	attrs, err := ParseNullableJSONMap(r.AttributesJSON)
	if err != nil {
		return inter.ExternalEntity{}, err
	}
	return inter.ExternalEntity{
		Source:      r.Source,
		EntityID:    r.EntityID,
		Domain:      r.Domain,
		GosterUUID:  r.GosterUUID,
		DeviceID:    r.DeviceID,
		Model:       r.Model,
		Name:        r.Name,
		RoomName:    r.RoomName,
		Unit:        r.Unit,
		ValueType:   r.ValueType,
		DeviceClass: r.DeviceClass,
		StateClass:  r.StateClass,
		Attributes:  attrs,
		LastStateTS: r.LastSeenTS,
		LastText:    nullableStringOut(r.LastStateText),
		LastNum:     nullableFloatOut(r.LastStateNum),
		LastBool:    NullIntToBoolPtr(r.LastStateBool),
	}, nil
}

func NewExternalObservationRow(item inter.ExternalObservation) (*ExternalObservationRow, error) {
	valueJSON, err := NullableJSONString(item.ValueJSON)
	if err != nil {
		return nil, err
	}
	rawEventJSON, err := NullableJSONString(item.RawEvent)
	if err != nil {
		return nil, err
	}
	return &ExternalObservationRow{
		TenantID:     DefaultTenantID,
		Source:       strings.TrimSpace(item.Source),
		EntityID:     strings.TrimSpace(item.EntityID),
		TS:           item.Timestamp,
		ValueNum:     NullableFloat64(item.ValueNum),
		ValueText:    NullableStringPtr(item.ValueText),
		ValueBool:    BoolPtrToNullableInt(item.ValueBool),
		ValueJSON:    valueJSON,
		Unit:         strings.TrimSpace(item.Unit),
		ValueSig:     strings.TrimSpace(item.ValueSig),
		RawEventJSON: rawEventJSON,
	}, nil
}

func (r ExternalObservationRow) ToExternalObservation() (inter.ExternalObservation, error) {
	valueJSON, err := ParseNullableJSONMap(r.ValueJSON)
	if err != nil {
		return inter.ExternalObservation{}, err
	}
	rawEvent, err := ParseNullableJSONMap(r.RawEventJSON)
	if err != nil {
		return inter.ExternalObservation{}, err
	}
	return inter.ExternalObservation{
		Source:    r.Source,
		EntityID:  r.EntityID,
		Timestamp: r.TS,
		ValueNum:  nullableFloatOut(r.ValueNum),
		ValueText: nullableStringOut(r.ValueText),
		ValueBool: NullIntToBoolPtr(r.ValueBool),
		ValueJSON: valueJSON,
		Unit:      r.Unit,
		ValueSig:  r.ValueSig,
		RawEvent:  rawEvent,
	}, nil
}
