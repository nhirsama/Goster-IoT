package datastore

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

func nullableJSONString(v map[string]interface{}) (interface{}, error) {
	if len(v) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func parseNullableJSONMap(raw sql.NullString) (map[string]interface{}, error) {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.String), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func boolPtrToNullableInt(v *bool) interface{} {
	if v == nil {
		return nil
	}
	if *v {
		return 1
	}
	return 0
}

func nullIntToBoolPtr(v sql.NullInt64) *bool {
	if !v.Valid {
		return nil
	}
	b := v.Int64 != 0
	return &b
}

func externalObservationSignature(item inter.ExternalObservation) string {
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

// UpsertExternalEntity 创建或更新外部实体主档
func (s *DataStoreSql) UpsertExternalEntity(entity inter.ExternalEntity) error {
	source := strings.TrimSpace(entity.Source)
	entityID := strings.TrimSpace(entity.EntityID)
	domain := strings.TrimSpace(entity.Domain)
	if source == "" || entityID == "" || domain == "" {
		return errors.New("source/entity_id/domain is required")
	}
	valueType := strings.TrimSpace(entity.ValueType)
	if valueType == "" {
		valueType = "string"
	}

	attrsJSON, err := nullableJSONString(entity.Attributes)
	if err != nil {
		return err
	}

	var lastNum interface{}
	if entity.LastNum != nil {
		lastNum = *entity.LastNum
	}
	var lastText interface{}
	if entity.LastText != nil {
		lastText = *entity.LastText
	}

	_, err = s.exec(`
		INSERT INTO integration_external_entities (
			source, entity_id, domain, goster_uuid, device_id, model, name, room_name,
			unit, value_type, device_class, state_class, attributes_json,
			last_state_text, last_state_num, last_state_bool, last_seen_ts
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source, entity_id) DO UPDATE SET
			domain = excluded.domain,
			goster_uuid = excluded.goster_uuid,
			device_id = excluded.device_id,
			model = excluded.model,
			name = excluded.name,
			room_name = excluded.room_name,
			unit = excluded.unit,
			value_type = excluded.value_type,
			device_class = excluded.device_class,
			state_class = excluded.state_class,
			attributes_json = excluded.attributes_json,
			last_state_text = excluded.last_state_text,
			last_state_num = excluded.last_state_num,
			last_state_bool = excluded.last_state_bool,
			last_seen_ts = excluded.last_seen_ts,
			updated_at = CURRENT_TIMESTAMP
	`,
		source, entityID, domain, entity.GosterUUID, entity.DeviceID, entity.Model, entity.Name, entity.RoomName,
		entity.Unit, valueType, entity.DeviceClass, entity.StateClass, attrsJSON,
		lastText, lastNum, boolPtrToNullableInt(entity.LastBool), entity.LastStateTS,
	)
	return err
}

// GetExternalEntity 查询单个外部实体
func (s *DataStoreSql) GetExternalEntity(source, entityID string) (inter.ExternalEntity, error) {
	var out inter.ExternalEntity
	var attrs sql.NullString
	var lastText sql.NullString
	var lastNum sql.NullFloat64
	var lastBool sql.NullInt64

	err := s.queryRow(`
		SELECT source, entity_id, domain, goster_uuid, device_id, model, name, room_name,
		       unit, value_type, device_class, state_class, attributes_json,
		       last_state_text, last_state_num, last_state_bool, last_seen_ts
		FROM integration_external_entities
		WHERE source = ? AND entity_id = ?
	`, source, entityID).Scan(
		&out.Source, &out.EntityID, &out.Domain, &out.GosterUUID, &out.DeviceID, &out.Model, &out.Name, &out.RoomName,
		&out.Unit, &out.ValueType, &out.DeviceClass, &out.StateClass, &attrs,
		&lastText, &lastNum, &lastBool, &out.LastStateTS,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, errors.New("external entity not found")
		}
		return out, err
	}

	if lastText.Valid {
		v := lastText.String
		out.LastText = &v
	}
	if lastNum.Valid {
		v := lastNum.Float64
		out.LastNum = &v
	}
	out.LastBool = nullIntToBoolPtr(lastBool)

	decodedAttrs, err := parseNullableJSONMap(attrs)
	if err != nil {
		return out, err
	}
	out.Attributes = decodedAttrs

	return out, nil
}

// ListExternalEntities 按 source/domain 分页查询外部实体
func (s *DataStoreSql) ListExternalEntities(source, domain string, limit, offset int) ([]inter.ExternalEntity, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	base := `
		SELECT source, entity_id, domain, goster_uuid, device_id, model, name, room_name,
		       unit, value_type, device_class, state_class, attributes_json,
		       last_state_text, last_state_num, last_state_bool, last_seen_ts
		FROM integration_external_entities
	`
	var cond []string
	var args []interface{}
	if strings.TrimSpace(source) != "" {
		cond = append(cond, "source = ?")
		args = append(args, source)
	}
	if strings.TrimSpace(domain) != "" {
		cond = append(cond, "domain = ?")
		args = append(args, domain)
	}
	if len(cond) > 0 {
		base += " WHERE " + strings.Join(cond, " AND ")
	}
	base += " ORDER BY last_seen_ts DESC, id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.query(base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]inter.ExternalEntity, 0, limit)
	for rows.Next() {
		var out inter.ExternalEntity
		var attrs sql.NullString
		var lastText sql.NullString
		var lastNum sql.NullFloat64
		var lastBool sql.NullInt64
		if err := rows.Scan(
			&out.Source, &out.EntityID, &out.Domain, &out.GosterUUID, &out.DeviceID, &out.Model, &out.Name, &out.RoomName,
			&out.Unit, &out.ValueType, &out.DeviceClass, &out.StateClass, &attrs,
			&lastText, &lastNum, &lastBool, &out.LastStateTS,
		); err != nil {
			return nil, err
		}
		if lastText.Valid {
			v := lastText.String
			out.LastText = &v
		}
		if lastNum.Valid {
			v := lastNum.Float64
			out.LastNum = &v
		}
		out.LastBool = nullIntToBoolPtr(lastBool)
		decodedAttrs, err := parseNullableJSONMap(attrs)
		if err != nil {
			return nil, err
		}
		out.Attributes = decodedAttrs
		items = append(items, out)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// BatchAppendExternalObservations 批量写入外部观测值。
// 这里统一使用 ON CONFLICT DO NOTHING，避免把 SQLite 方言写死在公共实现里。
func (s *DataStoreSql) BatchAppendExternalObservations(items []inter.ExternalObservation) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := s.begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := s.prepareTx(tx, `
		INSERT INTO integration_external_observations (
			source, entity_id, ts, value_num, value_text, value_bool, value_json, unit, value_sig, raw_event_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source, entity_id, ts, value_sig) DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		source := strings.TrimSpace(item.Source)
		entityID := strings.TrimSpace(item.EntityID)
		if source == "" || entityID == "" {
			return errors.New("source/entity_id is required")
		}
		ts := item.Timestamp
		if ts <= 0 {
			ts = time.Now().UnixMilli()
		}

		var numParam interface{}
		if item.ValueNum != nil {
			numParam = *item.ValueNum
		}
		var textParam interface{}
		if item.ValueText != nil {
			textParam = *item.ValueText
		}

		valueJSONParam, err := nullableJSONString(item.ValueJSON)
		if err != nil {
			return err
		}
		rawEventParam, err := nullableJSONString(item.RawEvent)
		if err != nil {
			return err
		}

		valueSig := strings.TrimSpace(item.ValueSig)
		if valueSig == "" {
			valueSig = externalObservationSignature(item)
		}

		if _, err := stmt.Exec(
			source, entityID, ts,
			numParam, textParam, boolPtrToNullableInt(item.ValueBool),
			valueJSONParam, item.Unit, valueSig, rawEventParam,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// QueryExternalObservations 查询外部观测值
func (s *DataStoreSql) QueryExternalObservations(source, entityID string, start, end int64, limit int) ([]inter.ExternalObservation, error) {
	if end <= 0 {
		end = time.Now().UnixMilli()
	}
	if start <= 0 || start > end {
		start = end - int64(24*time.Hour/time.Millisecond)
	}
	if limit <= 0 {
		limit = 1000
	}

	rows, err := s.query(`
		SELECT source, entity_id, ts, value_num, value_text, value_bool, value_json, unit, value_sig, raw_event_json
		FROM integration_external_observations
		WHERE source = ? AND entity_id = ? AND ts BETWEEN ? AND ?
		ORDER BY ts ASC
		LIMIT ?
	`, source, entityID, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]inter.ExternalObservation, 0, limit)
	for rows.Next() {
		var item inter.ExternalObservation
		var valueNum sql.NullFloat64
		var valueText sql.NullString
		var valueBool sql.NullInt64
		var valueJSON sql.NullString
		var rawEvent sql.NullString
		if err := rows.Scan(
			&item.Source, &item.EntityID, &item.Timestamp, &valueNum, &valueText, &valueBool, &valueJSON, &item.Unit, &item.ValueSig, &rawEvent,
		); err != nil {
			return nil, err
		}
		if valueNum.Valid {
			v := valueNum.Float64
			item.ValueNum = &v
		}
		if valueText.Valid {
			v := valueText.String
			item.ValueText = &v
		}
		item.ValueBool = nullIntToBoolPtr(valueBool)

		decodedValueJSON, err := parseNullableJSONMap(valueJSON)
		if err != nil {
			return nil, err
		}
		decodedRawEvent, err := parseNullableJSONMap(rawEvent)
		if err != nil {
			return nil, err
		}
		item.ValueJSON = decodedValueJSON
		item.RawEvent = decodedRawEvent
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func isValidDeviceCommandStatus(status inter.DeviceCommandStatus) bool {
	switch status {
	case inter.DeviceCommandStatusQueued, inter.DeviceCommandStatusSent, inter.DeviceCommandStatusAcked, inter.DeviceCommandStatusFailed:
		return true
	default:
		return false
	}
}

// CreateDeviceCommand 创建一条设备下行指令日志
func (s *DataStoreSql) CreateDeviceCommand(uuid string, cmdID inter.CmdID, command string, payloadJSON []byte) (int64, error) {
	tenantID, err := s.ResolveDeviceTenant(uuid)
	if err != nil {
		tenantID = defaultTenantID
	}
	return s.createDeviceCommandRecord(tenantID, uuid, cmdID, command, payloadJSON, false)
}

func (s *DataStoreSql) CreateDeviceCommandByTenant(tenantID, uuid string, cmdID inter.CmdID, command string, payloadJSON []byte) (int64, error) {
	return s.createDeviceCommandRecord(tenantID, uuid, cmdID, command, payloadJSON, true)
}

func (s *DataStoreSql) createDeviceCommandRecord(tenantID, uuid string, cmdID inter.CmdID, command string, payloadJSON []byte, validateTenant bool) (int64, error) {
	tenantID = normalizeTenantID(tenantID)
	uuid = strings.TrimSpace(uuid)
	command = strings.TrimSpace(strings.ToLower(command))
	if uuid == "" {
		return 0, errors.New("uuid is required")
	}
	if command == "" {
		return 0, errors.New("command is required")
	}
	if validateTenant {
		deviceTenant, err := s.ResolveDeviceTenant(uuid)
		if err != nil {
			return 0, err
		}
		if normalizeTenantID(deviceTenant) != tenantID {
			return 0, errors.New("device tenant mismatch")
		}
	}

	var payload interface{}
	trimmedPayload := strings.TrimSpace(string(payloadJSON))
	if trimmedPayload != "" {
		payload = trimmedPayload
	}

	commandID, err := s.insertReturningID(`
		INSERT INTO integration_external_commands (
			tenant_id, source, entity_id, command, payload_json, status, error_text, requested_at, executed_at
		) VALUES (?, ?, ?, ?, ?, ?, NULL, CURRENT_TIMESTAMP, NULL)
		RETURNING id
	`, tenantID, "goster_device", uuid, fmt.Sprintf("%s:%d", command, cmdID), payload, inter.DeviceCommandStatusQueued)
	if err != nil {
		return 0, err
	}
	return commandID, nil
}

// UpdateDeviceCommandStatus 更新设备下行指令状态
func (s *DataStoreSql) UpdateDeviceCommandStatus(commandID int64, status inter.DeviceCommandStatus, errorText string) error {
	if commandID <= 0 {
		return errors.New("invalid command id")
	}
	if !isValidDeviceCommandStatus(status) {
		return errors.New("invalid command status")
	}

	var errParam interface{}
	if strings.TrimSpace(errorText) != "" {
		errParam = strings.TrimSpace(errorText)
	}

	var executedAt interface{}
	if status == inter.DeviceCommandStatusAcked || status == inter.DeviceCommandStatusFailed {
		executedAt = time.Now().UTC()
	}

	result, err := s.exec(`
		UPDATE integration_external_commands
		SET status = ?, error_text = ?, executed_at = COALESCE(?, executed_at)
		WHERE id = ? AND source = ?
	`, status, errParam, executedAt, commandID, "goster_device")
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("device command not found")
	}
	return nil
}
