package DataStore

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	_ "modernc.org/sqlite"
)

type DataStoreSql struct {
	db *sql.DB
}

func NewDataStoreSql(dbPath string) (inter.DataStore, error) {
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

    CREATE TABLE IF NOT EXISTS logs (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       uuid TEXT,
       level TEXT,
       message TEXT,
       created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_logs_uuid ON logs (uuid);

    CREATE TABLE IF NOT EXISTS users (
       username TEXT PRIMARY KEY,
       password_hash TEXT,
       salt TEXT,
       permission INTEGER,
       created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    `

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &DataStoreSql{db: db}, nil
}

// InitDevice 将结构体字段拆解为 SQL 参数插入
func (s *DataStoreSql) InitDevice(uuid string, meta inter.DeviceMetadata) error {
	_, err := s.db.Exec(`
		INSERT INTO devices (uuid, name, hw_version, sw_version, config_version, sn, mac, created_at, token, auth_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid, meta.Name, meta.HWVersion, meta.SWVersion, meta.ConfigVersion,
		meta.SerialNumber, meta.MACAddress, time.Now(), meta.Token, meta.AuthenticateStatus,
	)
	return err
}

// LoadConfig 从独立列中读取数据并组装回结构体
func (s *DataStoreSql) LoadConfig(uuid string) (out inter.DeviceMetadata, err error) {
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

func (s *DataStoreSql) SaveMetadata(uuid string, meta inter.DeviceMetadata) error {
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
func (s *DataStoreSql) AppendMetric(uuid string, points inter.MetricPoint) error {
	_, err := s.db.Exec("INSERT INTO metrics (uuid, ts, value) VALUES (?, ?, ?)", uuid, points.Timestamp, points.Value)
	return err
}

func (s *DataStoreSql) QueryMetrics(uuid string, start, end int64) ([]inter.MetricPoint, error) {
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
func (s *DataStoreSql) GetDeviceByToken(token string) (string, inter.AuthenticateStatusType, error) {
	var uuid string
	var authStatus int // SQLite 中的 INTEGER 对应 Go 的 int

	// 在 SQL 语句中同时请求两列数据
	query := "SELECT uuid, auth_status FROM devices WHERE token = ?"

	// 执行查询并利用预处理语句防止 SQL 注入
	err := s.db.QueryRow(query, token).Scan(&uuid, &authStatus)

	if err != nil {
		if err == sql.ErrNoRows {
			// 如果找不到 Token，返回零值和特定错误
			return "", 0, fmt.Errorf("token not found: %w", err)
		}
		return "", 0, err
	}

	// 将数据库中的 int 强制转换为接口定义的 AuthenticateStatusType 类型
	return uuid, inter.AuthenticateStatusType(authStatus), nil
}

func (s *DataStoreSql) UpdateToken(uuid string, newToken string) error {
	_, err := s.db.Exec("UPDATE devices SET token = ? WHERE uuid = ?", newToken, uuid)
	return err
}

// DestroyDevice 物理删除设备及其所有关联数据
func (s *DataStoreSql) DestroyDevice(uuid string) error {
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
func (s *DataStoreSql) ListDevices(page, size int) ([]inter.DeviceRecord, error) {
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

// ListDevicesByStatus 根据认证状态分页查询设备列表
func (s *DataStoreSql) ListDevicesByStatus(status inter.AuthenticateStatusType, page, size int) ([]inter.DeviceRecord, error) {
	offset := (page - 1) * size
	rows, err := s.db.Query(`
        SELECT uuid, name, hw_version, sw_version, config_version, sn, mac, created_at, token, auth_status 
        FROM devices WHERE auth_status = ? LIMIT ? OFFSET ?`, status, size, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []inter.DeviceRecord
	for rows.Next() {
		var r inter.DeviceRecord
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
func (s *DataStoreSql) WriteLog(uuid string, level string, message string) error {
	// 插入日志，时间戳由数据库默认值生成
	_, err := s.db.Exec("INSERT INTO logs (uuid, level, message) VALUES (?, ?, ?)", uuid, level, message)
	return err
}

// BatchAppendMetrics 批量高效写入
func (s *DataStoreSql) BatchAppendMetrics(uuid string, points []inter.MetricPoint) error {
	// 开启事务
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	// 安全机制：如果函数中途崩溃或返回错误，自动回滚，保证数据一致性
	defer tx.Rollback()

	// 预编译 SQL 语句 (极大地提升循环插入的性能)
	stmt, err := tx.Prepare("INSERT INTO metrics (uuid, ts, value) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close() // 循环结束后关闭 statement

	// 在内存中执行循环插入
	for _, p := range points {
		if _, err := stmt.Exec(uuid, p.Timestamp, p.Value); err != nil {
			return err // 只要有一条失败，直接返回错误，defer 会触发 Rollback
		}
	}

	// 提交事务 (这是唯一一次真正的磁盘 IO)
	return tx.Commit()
}

// [用户管理实现]

// generateSalt 生成随机盐值
func (s *DataStoreSql) generateSalt() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// hashPassword 计算密码哈希：SHA256(password + salt)
func (s *DataStoreSql) hashPassword(password, salt string) string {
	hash := sha256.New()
	hash.Write([]byte(password + salt))
	return hex.EncodeToString(hash.Sum(nil))
}

// RegisterUser 注册新用户
func (s *DataStoreSql) RegisterUser(username, password string, permission inter.PermissionType) error {
	// 生成盐
	salt, err := s.generateSalt()
	if err != nil {
		return err
	}

	// 计算哈希
	hashed := s.hashPassword(password, salt)

	// 插入数据库
	_, err = s.db.Exec(`
		INSERT INTO users (username, password_hash, salt, permission)
		VALUES (?, ?, ?, ?)`,
		username, hashed, salt, permission,
	)
	if err != nil {
		// 检查唯一性约束冲突（SQLite error code 19 or generic string check）
		// 这里简单返回错误，上层可以根据错误信息判断是否是用户名已存在
		return fmt.Errorf("failed to register user: %w", err)
	}
	return nil
}

// LoginUser 用户登录
func (s *DataStoreSql) LoginUser(username, password string) (inter.PermissionType, error) {
	var currentHash, salt string
	var permission int

	// 查询用户凭证
	err := s.db.QueryRow("SELECT password_hash, salt, permission FROM users WHERE username = ?", username).Scan(&currentHash, &salt, &permission)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, errors.New("user not found or password incorrect")
		}
		return 0, err
	}

	// 验证密码
	if s.hashPassword(password, salt) != currentHash {
		return 0, errors.New("user not found or password incorrect")
	}

	return inter.PermissionType(permission), nil
}

// ChangePassword 修改密码
func (s *DataStoreSql) ChangePassword(username, oldPassword, newPassword string) error {
	// 查询现有用户的哈希和盐
	var currentHash, salt string
	err := s.db.QueryRow("SELECT password_hash, salt FROM users WHERE username = ?", username).Scan(&currentHash, &salt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("user not found")
		}
		return err
	}

	// 验证旧密码
	if s.hashPassword(oldPassword, salt) != currentHash {
		return errors.New("invalid old password")
	}

	// 生成新盐和新哈希
	newSalt, err := s.generateSalt()
	if err != nil {
		return err
	}
	newHash := s.hashPassword(newPassword, newSalt)
	// 更新数据库
	_, err = s.db.Exec("UPDATE users SET password_hash = ?, salt = ? WHERE username = ?", newHash, newSalt, username)
	return err
}
func (s *DataStoreSql) GetUserCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}
func (s *DataStoreSql) ListUsers() ([]inter.User, error) {
	rows, err := s.db.Query("SELECT username, permission, created_at FROM users")
	if err != nil {
		return nil, err

	}
	defer rows.Close()
	var users []inter.User
	for rows.Next() {
		var u inter.User
		var perm int
		if err := rows.Scan(&u.Username, &perm, &u.CreatedAt); err != nil {
			continue
		}
		u.Permission = inter.PermissionType(perm)
		users = append(users, u)
	}
	return users, nil
}

func (s *DataStoreSql) GetUserPermission(username string) (inter.PermissionType, error) {
	var perm int
	err := s.db.QueryRow("SELECT permission FROM users WHERE username = ?", username).Scan(&perm)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return inter.PermissionNone, errors.New("user not found")
		}
		return inter.PermissionNone, err
	}
	return inter.PermissionType(perm), nil
}
func (s *DataStoreSql) UpdateUserPermission(username string, perm inter.PermissionType) error {
	_, err := s.db.Exec("UPDATE users SET permission = ? WHERE username = ?", perm, username)
	return err
}
