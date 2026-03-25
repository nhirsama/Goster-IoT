package bunrepo

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/uptrace/bun"
)

const (
	DefaultTenantID = "tenant_legacy"
	LegacyTenantID  = "tenant_legacy"
)

type DeviceModel struct {
	bun.BaseModel `bun:"table:devices"`

	UUID          string         `bun:"uuid,pk"`
	TenantID      string         `bun:"tenant_id"`
	Name          string         `bun:"name"`
	HWVersion     string         `bun:"hw_version"`
	SWVersion     string         `bun:"sw_version"`
	ConfigVersion string         `bun:"config_version"`
	SerialNumber  string         `bun:"sn"`
	MACAddress    string         `bun:"mac"`
	CreatedAt     time.Time      `bun:"created_at"`
	Token         sql.NullString `bun:"token"`
	AuthStatus    int            `bun:"auth_status"`
}

type UserRow struct {
	bun.BaseModel `bun:"table:users"`

	ID         int64          `bun:"id,pk"`
	Username   sql.NullString `bun:"username"`
	Permission int            `bun:"permission"`
	CreatedAt  time.Time      `bun:"created_at"`
}

type TenantRoleRow struct {
	bun.BaseModel `bun:"table:tenant_users"`

	TenantID string `bun:"tenant_id"`
	Role     string `bun:"role"`
}

type MetricRow struct {
	bun.BaseModel `bun:"table:metrics"`

	UUID     string  `bun:"uuid"`
	TenantID string  `bun:"tenant_id"`
	TS       int64   `bun:"ts"`
	Value    float32 `bun:"value"`
	Type     uint8   `bun:"type"`
}

type LogRow struct {
	bun.BaseModel `bun:"table:logs"`

	ID        int64     `bun:"id,pk,autoincrement"`
	UUID      string    `bun:"uuid"`
	TenantID  string    `bun:"tenant_id"`
	Level     string    `bun:"level"`
	Message   string    `bun:"message"`
	CreatedAt time.Time `bun:"created_at"`
}

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

func NewDeviceModel(uuid string, meta inter.DeviceMetadata) *DeviceModel {
	return &DeviceModel{
		UUID:          uuid,
		TenantID:      DefaultTenantID,
		Name:          meta.Name,
		HWVersion:     meta.HWVersion,
		SWVersion:     meta.SWVersion,
		ConfigVersion: meta.ConfigVersion,
		SerialNumber:  meta.SerialNumber,
		MACAddress:    meta.MACAddress,
		CreatedAt:     time.Now(),
		Token:         NullableToken(meta.Token),
		AuthStatus:    int(meta.AuthenticateStatus),
	}
}

func (m DeviceModel) ToMetadata() inter.DeviceMetadata {
	out := inter.DeviceMetadata{
		Name:               m.Name,
		HWVersion:          m.HWVersion,
		SWVersion:          m.SWVersion,
		ConfigVersion:      m.ConfigVersion,
		SerialNumber:       m.SerialNumber,
		MACAddress:         m.MACAddress,
		CreatedAt:          m.CreatedAt,
		AuthenticateStatus: inter.AuthenticateStatusType(m.AuthStatus),
	}
	if m.Token.Valid {
		out.Token = m.Token.String
	}
	return out
}

func (m DeviceModel) ToRecord() inter.DeviceRecord {
	return inter.DeviceRecord{
		UUID: m.UUID,
		Meta: m.ToMetadata(),
	}
}

func NullableToken(token string) sql.NullString {
	token = strings.TrimSpace(token)
	if token == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: token, Valid: true}
}

func NormalizeTenantRole(raw string) inter.TenantRole {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(inter.TenantRoleAdmin):
		return inter.TenantRoleAdmin
	case string(inter.TenantRoleRW):
		return inter.TenantRoleRW
	default:
		return inter.TenantRoleRO
	}
}

func PermissionToTenantRole(perm inter.PermissionType) inter.TenantRole {
	switch perm {
	case inter.PermissionAdmin:
		return inter.TenantRoleAdmin
	case inter.PermissionReadWrite:
		return inter.TenantRoleRW
	default:
		return inter.TenantRoleRO
	}
}

func NormalizeTenantID(tenantID string) string {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return DefaultTenantID
	}
	return tenantID
}

func ToMetricPoints(rows []MetricRow) []inter.MetricPoint {
	out := make([]inter.MetricPoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, inter.MetricPoint{
			Timestamp: row.TS,
			Value:     row.Value,
			Type:      row.Type,
		})
	}
	return out
}

func NullableJSONString(v map[string]interface{}) (sql.NullString, error) {
	if len(v) == 0 {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

func ParseNullableJSONMap(raw sql.NullString) (map[string]interface{}, error) {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.String), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func BoolPtrToNullableInt(v *bool) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	if *v {
		return sql.NullInt64{Int64: 1, Valid: true}
	}
	return sql.NullInt64{Int64: 0, Valid: true}
}

func NullIntToBoolPtr(v sql.NullInt64) *bool {
	if !v.Valid {
		return nil
	}
	b := v.Int64 != 0
	return &b
}

func NullableFloat64(v *float64) sql.NullFloat64 {
	if v == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *v, Valid: true}
}

func NullableStringPtr(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	if strings.TrimSpace(*v) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

func PayloadStringPtr(payload []byte) *string {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func NullableOptionalString(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func ExternalObservationSignature(item inter.ExternalObservation) string {
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

func nullableFloatOut(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	out := v.Float64
	return &out
}

func nullableStringOut(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	out := v.String
	return &out
}
