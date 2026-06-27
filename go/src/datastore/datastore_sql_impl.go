package datastore

import (
	"database/sql"

	_ "github.com/lib/pq"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

type DataStoreSql struct {
	db *sql.DB
	// Prepared statement cache for frequently used queries
	stmts struct {
		queryMetrics          *sql.Stmt
		queryMetricsWithLimit *sql.Stmt
		insertMetric          *sql.Stmt
		insertDevice          *sql.Stmt
		updateDeviceMeta      *sql.Stmt
		getDeviceByToken      *sql.Stmt
		loadConfig            *sql.Stmt
	}
}

func NewDataStoreSql(databaseURL string) (inter.DataStore, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	// 配置连接池参数以提升性能
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(0)

	if err := ensureBaseSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	if err := runSchemaMigrations(db); err != nil {
		db.Close()
		return nil, err
	}

	ds := &DataStoreSql{db: db}

	// 初始化预编译语句缓存
	if err := ds.initPreparedStatements(); err != nil {
		db.Close()
		return nil, err
	}

	return ds, nil
}

// initPreparedStatements 初始化常用查询的预编译语句
func (s *DataStoreSql) initPreparedStatements() error {
	var err error

	// 指标查询（无限制）- PostgreSQL uses $1, $2, $3 for placeholders
	s.stmts.queryMetrics, err = s.db.Prepare(
		"SELECT ts, value, type FROM metrics WHERE uuid = $1 AND ts BETWEEN $2 AND $3 ORDER BY ts ASC",
	)
	if err != nil {
		return err
	}

	// 指标查询（带 LIMIT）
	s.stmts.queryMetricsWithLimit, err = s.db.Prepare(
		"SELECT ts, value, type FROM metrics WHERE uuid = $1 AND ts BETWEEN $2 AND $3 ORDER BY ts ASC LIMIT $4",
	)
	if err != nil {
		return err
	}

	// 插入指标
	s.stmts.insertMetric, err = s.db.Prepare(
		"INSERT INTO metrics (uuid, tenant_id, ts, value, type) VALUES ($1, $2, $3, $4, $5)",
	)
	if err != nil {
		return err
	}

	// 插入设备
	s.stmts.insertDevice, err = s.db.Prepare(
		`INSERT INTO devices (uuid, tenant_id, name, hw_version, sw_version, config_version, sn, mac, created_at, token, auth_status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
	)
	if err != nil {
		return err
	}

	// 更新设备元数据
	s.stmts.updateDeviceMeta, err = s.db.Prepare(
		`UPDATE devices SET name=$1, hw_version=$2, sw_version=$3, config_version=$4, sn=$5, mac=$6, auth_status=$7, token=$8 WHERE uuid=$9`,
	)
	if err != nil {
		return err
	}

	// 根据 Token 查询设备
	s.stmts.getDeviceByToken, err = s.db.Prepare(
		"SELECT uuid, auth_status FROM devices WHERE token = $1",
	)
	if err != nil {
		return err
	}

	// 加载设备配置
	s.stmts.loadConfig, err = s.db.Prepare(
		"SELECT name, hw_version, sw_version, config_version, sn, mac, created_at, token, auth_status FROM devices WHERE uuid = $1",
	)
	if err != nil {
		return err
	}

	return nil
}

func ensureBaseSchema(db *sql.DB) error {
	// 初始化原子化表结构 - PostgreSQL syntax
	schema := `
    CREATE TABLE IF NOT EXISTS devices (
       uuid TEXT PRIMARY KEY,
       tenant_id TEXT DEFAULT 'tenant_legacy',
       name TEXT,
       hw_version TEXT,
       sw_version TEXT,
       config_version TEXT,
       sn TEXT,
       mac TEXT,
       created_at TIMESTAMP,
       token TEXT UNIQUE,
       auth_status INTEGER
    );

	CREATE TABLE IF NOT EXISTS metrics (
	   uuid TEXT,
	   tenant_id TEXT DEFAULT 'tenant_legacy',
	   ts BIGINT,
	   value REAL,
	   type INTEGER DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_metrics_query ON metrics (uuid, ts);

    CREATE TABLE IF NOT EXISTS logs (
       id SERIAL PRIMARY KEY,
       uuid TEXT,
       tenant_id TEXT DEFAULT 'tenant_legacy',
       level TEXT,
       message TEXT,
       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_logs_uuid ON logs (uuid);

    -- New Authboss-compatible Schema
    CREATE TABLE IF NOT EXISTS users (
       id            SERIAL PRIMARY KEY,
       email         TEXT,
       username      TEXT UNIQUE NOT NULL,
       password      TEXT NOT NULL,
       permission    INTEGER DEFAULT 0,

       oauth2_uid           TEXT,
       oauth2_provider      TEXT,
       oauth2_access_token  TEXT,
       oauth2_refresh_token TEXT,
       oauth2_expiry        TIMESTAMP,
       remember_token       TEXT,

       recover_token        TEXT,
       recover_token_expiry TIMESTAMP,

       confirm_token        TEXT,
       confirmed            BOOLEAN DEFAULT FALSE,

       last_login           TIMESTAMP,
       created_at           TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
       updated_at           TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

    -- External integration entities (e.g. Xiaomi via Home Assistant)
    CREATE TABLE IF NOT EXISTS integration_external_entities (
       id SERIAL PRIMARY KEY,
       tenant_id TEXT DEFAULT 'tenant_legacy',
       source TEXT NOT NULL,
       entity_id TEXT NOT NULL,
       domain TEXT NOT NULL,
       goster_uuid TEXT,
       device_id TEXT,
       model TEXT,
       name TEXT,
       room_name TEXT,
       unit TEXT,
       value_type TEXT NOT NULL DEFAULT 'string',
       device_class TEXT,
       state_class TEXT,
       attributes_json TEXT,
       last_state_text TEXT,
       last_state_num REAL,
       last_state_bool INTEGER,
       last_seen_ts BIGINT,
       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
       updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
       UNIQUE(source, entity_id)
    );
    CREATE INDEX IF NOT EXISTS idx_integration_entities_source_domain
      ON integration_external_entities (source, domain);
    CREATE INDEX IF NOT EXISTS idx_integration_entities_uuid
      ON integration_external_entities (goster_uuid);

    -- External observations that can carry numeric/bool/text/json values
    CREATE TABLE IF NOT EXISTS integration_external_observations (
       id SERIAL PRIMARY KEY,
       tenant_id TEXT DEFAULT 'tenant_legacy',
       source TEXT NOT NULL,
       entity_id TEXT NOT NULL,
       ts BIGINT NOT NULL,
       value_num REAL,
       value_text TEXT,
       value_bool INTEGER,
       value_json TEXT,
       unit TEXT,
       value_sig TEXT NOT NULL DEFAULT '',
       raw_event_json TEXT,
       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
       UNIQUE(source, entity_id, ts, value_sig)
    );
    CREATE INDEX IF NOT EXISTS idx_integration_observations_query
      ON integration_external_observations (source, entity_id, ts);

    -- Optional command log for future control write-back
    CREATE TABLE IF NOT EXISTS integration_external_commands (
       id SERIAL PRIMARY KEY,
       tenant_id TEXT DEFAULT 'tenant_legacy',
       source TEXT NOT NULL,
       entity_id TEXT NOT NULL,
       command TEXT NOT NULL,
       payload_json TEXT,
       status TEXT NOT NULL DEFAULT 'pending',
       error_text TEXT,
       requested_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
       executed_at TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_integration_commands_status
      ON integration_external_commands (source, status, requested_at);

    CREATE TABLE IF NOT EXISTS tenants (
       id TEXT PRIMARY KEY,
       name TEXT NOT NULL UNIQUE,
       status TEXT NOT NULL DEFAULT 'active',
       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
       updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS tenant_users (
       tenant_id TEXT NOT NULL,
       username TEXT NOT NULL,
       role TEXT NOT NULL,
       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
       PRIMARY KEY (tenant_id, username),
       FOREIGN KEY (tenant_id) REFERENCES tenants(id)
    );
    CREATE INDEX IF NOT EXISTS idx_tenant_users_username ON tenant_users (username);

    CREATE TABLE IF NOT EXISTS device_groups (
       id TEXT PRIMARY KEY,
       tenant_id TEXT NOT NULL,
       name TEXT NOT NULL,
       description TEXT DEFAULT '',
       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
       updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
       UNIQUE (tenant_id, name),
       FOREIGN KEY (tenant_id) REFERENCES tenants(id)
    );
    CREATE INDEX IF NOT EXISTS idx_device_groups_tenant ON device_groups (tenant_id);

    CREATE TABLE IF NOT EXISTS group_devices (
       group_id TEXT NOT NULL,
       device_uuid TEXT NOT NULL,
       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
       PRIMARY KEY (group_id, device_uuid),
       FOREIGN KEY (group_id) REFERENCES device_groups(id)
    );
    CREATE INDEX IF NOT EXISTS idx_group_devices_uuid ON group_devices (device_uuid);

    CREATE TABLE IF NOT EXISTS group_users (
       group_id TEXT NOT NULL,
       username TEXT NOT NULL,
       role TEXT NOT NULL,
       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
       PRIMARY KEY (group_id, username),
       FOREIGN KEY (group_id) REFERENCES device_groups(id)
    );
    CREATE INDEX IF NOT EXISTS idx_group_users_username ON group_users (username);
    `

	if _, err := db.Exec(schema); err != nil {
		return err
	}
	return nil
}

func runSchemaMigrations(db *sql.DB) error {
	// 兼容旧库结构：为历史数据库补齐多租户字段。
	// PostgreSQL: ALTER TABLE ADD COLUMN IF NOT EXISTS is supported in PostgreSQL 9.6+
	columnMigrations := []string{
		"ALTER TABLE metrics ADD COLUMN IF NOT EXISTS type INTEGER DEFAULT 0",
		"ALTER TABLE devices ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE metrics ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE logs ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE integration_external_entities ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE integration_external_observations ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE integration_external_commands ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'tenant_legacy'",
	}
	for _, stmt := range columnMigrations {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	postMigrations := []string{
		"CREATE INDEX IF NOT EXISTS idx_metrics_type ON metrics (uuid, type, ts)",
		"CREATE INDEX IF NOT EXISTS idx_devices_tenant_uuid ON devices (tenant_id, uuid)",
		"CREATE INDEX IF NOT EXISTS idx_metrics_tenant_uuid_ts ON metrics (tenant_id, uuid, ts)",
		"CREATE INDEX IF NOT EXISTS idx_logs_tenant_uuid_created ON logs (tenant_id, uuid, created_at)",
		"CREATE INDEX IF NOT EXISTS idx_ext_entities_tenant_source_domain ON integration_external_entities (tenant_id, source, domain)",
		"CREATE INDEX IF NOT EXISTS idx_ext_obs_tenant_source_entity_ts ON integration_external_observations (tenant_id, source, entity_id, ts)",
		"CREATE INDEX IF NOT EXISTS idx_ext_cmd_tenant_source_status ON integration_external_commands (tenant_id, source, status, requested_at)",
		"INSERT INTO tenants (id, name, status) VALUES ('tenant_legacy', 'legacy', 'active') ON CONFLICT (id) DO NOTHING",
		"UPDATE devices SET tenant_id = 'tenant_legacy' WHERE tenant_id IS NULL OR TRIM(tenant_id) = ''",
		"UPDATE metrics SET tenant_id = 'tenant_legacy' WHERE tenant_id IS NULL OR TRIM(tenant_id) = ''",
		"UPDATE logs SET tenant_id = 'tenant_legacy' WHERE tenant_id IS NULL OR TRIM(tenant_id) = ''",
		"UPDATE integration_external_entities SET tenant_id = 'tenant_legacy' WHERE tenant_id IS NULL OR TRIM(tenant_id) = ''",
		"UPDATE integration_external_observations SET tenant_id = 'tenant_legacy' WHERE tenant_id IS NULL OR TRIM(tenant_id) = ''",
		"UPDATE integration_external_commands SET tenant_id = 'tenant_legacy' WHERE tenant_id IS NULL OR TRIM(tenant_id) = ''",
		`INSERT INTO tenant_users (tenant_id, username, role)
		 SELECT 'tenant_legacy', username,
		        CASE
		          WHEN permission >= 3 THEN 'tenant_admin'
		          WHEN permission = 2 THEN 'tenant_rw'
		          ELSE 'tenant_ro'
		        END
		   FROM users
		  WHERE username IS NOT NULL AND TRIM(username) <> ''
		 ON CONFLICT (tenant_id, username) DO NOTHING`,
	}
	for _, stmt := range postMigrations {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
