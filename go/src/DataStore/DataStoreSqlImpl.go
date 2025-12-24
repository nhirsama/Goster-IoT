package DataStore

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	_ "modernc.org/sqlite"
)

type AtomStore struct {
	db *sql.DB
}

func NewAtomStore(dbPath string) (inter.DataStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// 初始化原子化表结构
	schema := `
	CREATE TABLE IF NOT EXISTS devices (
		uuid TEXT PRIMARY KEY,
		name TEXT,
		hw_version TEXT,
		sw_version TEXT,
		config_version TEXT,
		sn TEXT,
		mac TEXT,
		created_at DATETIME,
		token TEXT UNIQUE,
		auth_status INTEGER
	);
	CREATE TABLE IF NOT EXISTS metrics (
		uuid TEXT,
		ts BIGINT,
		value REAL
	);
	CREATE INDEX IF NOT EXISTS idx_metrics_query ON metrics (uuid, ts);
	`
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return &AtomStore{db: db}, nil
}

// InitDevice 将结构体字段拆解为 SQL 参数插入
func (s *AtomStore) InitDevice(uuid string, meta inter.DeviceMetadata) error {
	_, err := s.db.Exec(`
		INSERT INTO devices (uuid, name, hw_version, sw_version, config_version, sn, mac, created_at, token, auth_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid, meta.Name, meta.HWVersion, meta.SWVersion, meta.ConfigVersion,
		meta.SerialNumber, meta.MACAddress, time.Now(), meta.Token, meta.AuthenticateStatus,
	)
	return err
}

// LoadConfig 从独立列中读取数据并组装回结构体
func (s *AtomStore) LoadConfig(uuid string) (out inter.DeviceMetadata, err error) {
	err = s.db.QueryRow(`
		SELECT name, hw_version, sw_version, config_version, sn, mac, created_at, token, auth_status 
		FROM devices WHERE uuid = ?`, uuid).Scan(
		&out.Name, &out.HWVersion, &out.SWVersion, &out.ConfigVersion,
		&out.SerialNumber, &out.MACAddress, &out.CreatedAt, &out.Token, &out.AuthenticateStatus,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return out, errors.New("device not found")
	}
	return out, err
}

func (s *AtomStore) SaveMetadata(uuid string, meta inter.DeviceMetadata) error {
	_, err := s.db.Exec(`
		UPDATE devices SET 
			name=?, hw_version=?, sw_version=?, config_version=?, sn=?, mac=?, auth_status=?
		WHERE uuid=?`,
		meta.Name, meta.HWVersion, meta.SWVersion, meta.ConfigVersion,
		meta.SerialNumber, meta.MACAddress, meta.AuthenticateStatus, uuid,
	)
	return err
}

// AppendMetric 插入传感器采样点
func (s *AtomStore) AppendMetric(uuid string, ts int64, value float32) error {
	_, err := s.db.Exec("INSERT INTO metrics (uuid, ts, value) VALUES (?, ?, ?)", uuid, ts, value)
	return err
}

func (s *AtomStore) QueryMetrics(uuid string, start, end int64) ([]inter.MetricPoint, error) {
	rows, err := s.db.Query(
		"SELECT ts, value FROM metrics WHERE uuid = ? AND ts BETWEEN ? AND ? ORDER BY ts ASC",
		uuid, start, end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []inter.MetricPoint
	for rows.Next() {
		var p inter.MetricPoint
		rows.Scan(&p.Timestamp, &p.Value)
		points = append(points, p)
	}
	return points, nil
}

// GetDeviceByToken 直接利用 token 上的唯一索引，速度极快
// GetDeviceByToken 实现了双字段查询：UUID 和 AuthStatus
func (s *AtomStore) GetDeviceByToken(token string) (string, inter.AuthenticateStatusType, error) {
	var uuid string
	var authStatus int // SQLite 中的 INTEGER 对应 Go 的 int

	// 1. 在 SQL 语句中同时请求两列数据
	query := "SELECT uuid, auth_status FROM devices WHERE token = ?"

	// 2. 执行查询并利用预处理语句防止 SQL 注入
	err := s.db.QueryRow(query, token).Scan(&uuid, &authStatus)

	if err != nil {
		if err == sql.ErrNoRows {
			// 如果找不到 Token，返回零值和特定错误
			return "", 0, fmt.Errorf("token not found: %w", err)
		}
		return "", 0, err
	}

	// 3. 将数据库中的 int 强制转换为接口定义的 AuthenticateStatusType 类型
	return uuid, inter.AuthenticateStatusType(authStatus), nil
}

func (s *AtomStore) UpdateToken(uuid string, newToken string) error {
	_, err := s.db.Exec("UPDATE devices SET token = ? WHERE uuid = ?", newToken, uuid)
	return err
}

// DestroyDevice 物理删除设备及其所有关联数据
func (s *AtomStore) DestroyDevice(uuid string) error {
	// 开启事务确保删除操作的原子性
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // 如果中间出错则回滚

	// 分别删除三张表中的相关数据
	if _, err := tx.Exec("DELETE FROM devices WHERE uuid = ?", uuid); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM metrics WHERE uuid = ?", uuid); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM logs WHERE uuid = ?", uuid); err != nil {
		return err
	}

	return tx.Commit()
}

// ListDevices 分页查询设备列表并组装结构体
func (s *AtomStore) ListDevices(page, size int) ([]inter.DeviceRecord, error) {
	offset := (page - 1) * size
	// 查询所有原子化字段
	rows, err := s.db.Query(`
        SELECT uuid, name, hw_version, sw_version, config_version, sn, mac, created_at, token, auth_status 
        FROM devices LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []inter.DeviceRecord
	for rows.Next() {
		var r inter.DeviceRecord
		// 逐个字段 Scan 到结构体中
		err := rows.Scan(
			&r.UUID, &r.Meta.Name, &r.Meta.HWVersion, &r.Meta.SWVersion,
			&r.Meta.ConfigVersion, &r.Meta.SerialNumber, &r.Meta.MACAddress,
			&r.Meta.CreatedAt, &r.Meta.Token, &r.Meta.AuthenticateStatus,
		)
		if err != nil {
			continue
		}
		records = append(records, r)
	}
	return records, nil
}

// WriteLog 记录设备运行日志
func (s *AtomStore) WriteLog(uuid string, level string, message string) error {
	// 插入日志，时间戳由数据库默认值生成
	_, err := s.db.Exec("INSERT INTO logs (uuid, level, message) VALUES (?, ?, ?)", uuid, level, message)
	return err
}
