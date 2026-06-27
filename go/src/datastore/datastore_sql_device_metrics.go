package datastore

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// InitDevice 将结构体字段拆解为 SQL 参数插入
func (s *DataStoreSql) InitDevice(uuid string, meta inter.DeviceMetadata) error {
	// 处理 Token：空字符串转为 NULL，避免违反 UNIQUE 约束
	var tokenParam interface{}
	if meta.Token == "" {
		tokenParam = nil
	} else {
		tokenParam = meta.Token
	}

	_, err := s.stmts.insertDevice.Exec(
		uuid, defaultTenantID, meta.Name, meta.HWVersion, meta.SWVersion, meta.ConfigVersion,
		meta.SerialNumber, meta.MACAddress, time.Now(), tokenParam, meta.AuthenticateStatus,
	)
	return err
}

// LoadConfig 从独立列中读取数据并组装回结构体
func (s *DataStoreSql) LoadConfig(uuid string) (out inter.DeviceMetadata, err error) {
	var token sql.NullString // 使用 NullString 接收数据库中的 NULL 值

	err = s.stmts.loadConfig.QueryRow(uuid).Scan(
		&out.Name, &out.HWVersion, &out.SWVersion, &out.ConfigVersion,
		&out.SerialNumber, &out.MACAddress, &out.CreatedAt, &token, &out.AuthenticateStatus,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return out, errors.New("device not found")
	}

	if token.Valid {
		out.Token = token.String
	} else {
		out.Token = ""
	}

	return out, err
}

func (s *DataStoreSql) SaveMetadata(uuid string, meta inter.DeviceMetadata) error {
	// 处理 Token：空字符串转为数据库 NULL
	var tokenParam interface{}
	if meta.Token == "" {
		tokenParam = nil
	} else {
		tokenParam = meta.Token
	}

	_, err := s.stmts.updateDeviceMeta.Exec(
		meta.Name, meta.HWVersion, meta.SWVersion, meta.ConfigVersion,
		meta.SerialNumber, meta.MACAddress, meta.AuthenticateStatus, tokenParam, uuid,
	)
	return err
}

// AppendMetric 插入传感器采样点
func (s *DataStoreSql) AppendMetric(uuid string, points inter.MetricPoint) error {
	tenantID, err := s.ResolveDeviceTenant(uuid)
	if err != nil {
		tenantID = defaultTenantID
	}
	_, err = s.stmts.insertMetric.Exec(uuid, tenantID, points.Timestamp, points.Value, points.Type)
	return err
}

func (s *DataStoreSql) QueryMetrics(uuid string, start, end int64, limit int) ([]inter.MetricPoint, error) {
	// 标准化 limit：0 使用默认值，-1 表示无限制（不推荐），否则限制在合理范围内
	if limit == 0 {
		limit = 1000 // 默认返回 1000 条
	} else if limit > 10000 {
		limit = 10000 // 最多返回 10000 条，防止 OOM
	}

	var rows *sql.Rows
	var err error

	if limit == -1 {
		// 无限制查询（不推荐，仅用于特殊场景）
		rows, err = s.stmts.queryMetrics.Query(uuid, start, end)
	} else {
		rows, err = s.stmts.queryMetricsWithLimit.Query(uuid, start, end, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []inter.MetricPoint
	for rows.Next() {
		var p inter.MetricPoint
		if err := rows.Scan(&p.Timestamp, &p.Value, &p.Type); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

func (s *DataStoreSql) ResolveDeviceTenant(uuid string) (string, error) {
	var tenantID sql.NullString
	err := s.db.QueryRow("SELECT tenant_id FROM devices WHERE uuid = $1", uuid).Scan(&tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("device not found")
		}
		return "", err
	}
	if tenantID.Valid && strings.TrimSpace(tenantID.String) != "" {
		return strings.TrimSpace(tenantID.String), nil
	}
	return defaultTenantID, nil
}

func (s *DataStoreSql) LoadConfigByTenant(tenantID, uuid string) (out inter.DeviceMetadata, err error) {
	tenantID = normalizeTenantID(tenantID)
	var token sql.NullString
	err = s.db.QueryRow(`
		SELECT name, hw_version, sw_version, config_version, sn, mac, created_at, token, auth_status
		FROM devices WHERE uuid = $1 AND tenant_id = $2`, uuid, tenantID).Scan(
		&out.Name, &out.HWVersion, &out.SWVersion, &out.ConfigVersion,
		&out.SerialNumber, &out.MACAddress, &out.CreatedAt, &token, &out.AuthenticateStatus,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return out, errors.New("device not found")
	}
	if token.Valid {
		out.Token = token.String
	}
	return out, err
}

func (s *DataStoreSql) ListDevicesByTenant(tenantID string, status *inter.AuthenticateStatusType, page, size int) ([]inter.DeviceRecord, error) {
	tenantID = normalizeTenantID(tenantID)
	offset := (page - 1) * size

	if status == nil {
		rows, err := s.db.Query(`
			SELECT uuid, name, hw_version, sw_version, config_version, sn, mac, created_at, token, auth_status
			FROM devices WHERE tenant_id = $1 LIMIT $2 OFFSET $3`, tenantID, size, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanDeviceRecords(rows)
	}

	rows, err := s.db.Query(`
		SELECT uuid, name, hw_version, sw_version, config_version, sn, mac, created_at, token, auth_status
		FROM devices WHERE tenant_id = $1 AND auth_status = $2 LIMIT $3 OFFSET $4`, tenantID, *status, size, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeviceRecords(rows)
}

func (s *DataStoreSql) QueryMetricsByTenant(tenantID, uuid string, start, end int64, limit int) ([]inter.MetricPoint, error) {
	tenantID = normalizeTenantID(tenantID)

	// 标准化 limit：0 使用默认值，-1 表示无限制（不推荐），否则限制在合理范围内
	if limit == 0 {
		limit = 1000 // 默认返回 1000 条
	} else if limit > 10000 {
		limit = 10000 // 最多返回 10000 条，防止 OOM
	}

	var rows *sql.Rows
	var err error

	if limit == -1 {
		// 无限制查询（不推荐，仅用于特殊场景）
		rows, err = s.db.Query(
			"SELECT ts, value, type FROM metrics WHERE tenant_id = $1 AND uuid = $2 AND ts BETWEEN $3 AND $4 ORDER BY ts ASC",
			tenantID, uuid, start, end,
		)
	} else {
		rows, err = s.db.Query(
			"SELECT ts, value, type FROM metrics WHERE tenant_id = $1 AND uuid = $2 AND ts BETWEEN $3 AND $4 ORDER BY ts ASC LIMIT $5",
			tenantID, uuid, start, end, limit,
		)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []inter.MetricPoint
	for rows.Next() {
		var p inter.MetricPoint
		if err := rows.Scan(&p.Timestamp, &p.Value, &p.Type); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

// GetDeviceByToken 直接利用 token 上的唯一索引，速度极快
// GetDeviceByToken 实现了双字段查询：UUID 和 AuthStatus
func (s *DataStoreSql) GetDeviceByToken(token string) (string, inter.AuthenticateStatusType, error) {
	var uuid string
	var authStatus int // SQLite 中的 INTEGER 对应 Go 的 int

	err := s.stmts.getDeviceByToken.QueryRow(token).Scan(&uuid, &authStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", 0, fmt.Errorf("token not found: %w", err)
		}
		return "", 0, err
	}

	return uuid, inter.AuthenticateStatusType(authStatus), nil
}

func (s *DataStoreSql) UpdateToken(uuid string, newToken string) error {
	res, err := s.db.Exec("UPDATE devices SET token = $1 WHERE uuid = $2", newToken, uuid)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("device not found")
	}
	return nil
}

// DestroyDevice 物理删除设备及其所有关联数据
func (s *DataStoreSql) DestroyDevice(uuid string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	deviceRes, err := tx.Exec("DELETE FROM devices WHERE uuid = $1", uuid)
	if err != nil {
		return err
	}
	deviceRows, err := deviceRes.RowsAffected()
	if err != nil {
		return err
	}
	if deviceRows == 0 {
		return errors.New("device not found")
	}
	if _, err := tx.Exec("DELETE FROM metrics WHERE uuid = $1", uuid); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM logs WHERE uuid = $1", uuid); err != nil {
		return err
	}

	return tx.Commit()
}

// ListDevices 分页查询设备列表并组装结构体
func (s *DataStoreSql) ListDevices(page, size int) ([]inter.DeviceRecord, error) {
	offset := (page - 1) * size
	rows, err := s.db.Query(`
        SELECT uuid, name, hw_version, sw_version, config_version, sn, mac, created_at, token, auth_status
        FROM devices LIMIT $1 OFFSET $2`, size, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeviceRecords(rows)
}

// ListDevicesByStatus 根据认证状态分页查询设备列表
func (s *DataStoreSql) ListDevicesByStatus(status inter.AuthenticateStatusType, page, size int) ([]inter.DeviceRecord, error) {
	offset := (page - 1) * size
	rows, err := s.db.Query(`
        SELECT uuid, name, hw_version, sw_version, config_version, sn, mac, created_at, token, auth_status
        FROM devices WHERE auth_status = $1 LIMIT $2 OFFSET $3`, status, size, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeviceRecords(rows)
}

// WriteLog 记录设备运行日志
func (s *DataStoreSql) WriteLog(uuid string, level string, message string) error {
	tenantID, err := s.ResolveDeviceTenant(uuid)
	if err != nil {
		tenantID = defaultTenantID
	}
	_, err = s.db.Exec("INSERT INTO logs (uuid, tenant_id, level, message) VALUES ($1, $2, $3, $4)", uuid, tenantID, level, message)
	return err
}

// BatchAppendMetrics 批量高效写入
func (s *DataStoreSql) BatchAppendMetrics(uuid string, points []inter.MetricPoint) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tenantID, tenantErr := s.ResolveDeviceTenant(uuid)
	if tenantErr != nil {
		tenantID = defaultTenantID
	}

	stmt, err := tx.Prepare("INSERT INTO metrics (uuid, tenant_id, ts, value, type) VALUES ($1, $2, $3, $4, $5)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range points {
		if _, err := stmt.Exec(uuid, tenantID, p.Timestamp, p.Value, p.Type); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func scanDeviceRecords(rows *sql.Rows) ([]inter.DeviceRecord, error) {
	var records []inter.DeviceRecord
	for rows.Next() {
		var r inter.DeviceRecord
		var token sql.NullString
		err := rows.Scan(
			&r.UUID, &r.Meta.Name, &r.Meta.HWVersion, &r.Meta.SWVersion,
			&r.Meta.ConfigVersion, &r.Meta.SerialNumber, &r.Meta.MACAddress,
			&r.Meta.CreatedAt, &token, &r.Meta.AuthenticateStatus,
		)
		if err != nil {
			return nil, err
		}
		if token.Valid {
			r.Meta.Token = token.String
		} else {
			r.Meta.Token = ""
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}
