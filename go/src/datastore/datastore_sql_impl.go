package datastore

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nhirsama/Goster-IoT/src/inter"
	_ "modernc.org/sqlite"
)

type DataStoreSql struct {
	db      *sql.DB
	dialect sqlDialect
}

// OpenDataStoreSql 只打开现有的 SQLite 数据库，不隐式建表。
func OpenDataStoreSql(dbPath string) (inter.DataStore, error) {
	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		return nil, err
	}
	return openSQLStore(db, dialectSQLite)
}

func NewDataStoreSql(dbPath string) (inter.DataStore, error) {
	// 强制指定 _loc=Local，确保 DATETIME 字段按本地时区读写，防止时区转换偏移
	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		return nil, err
	}

	return newSQLStore(db, dialectSQLite)
}

// OpenDataStorePostgres 只打开现有的 PostgreSQL 数据库，不隐式建表。
func OpenDataStorePostgres(dsn string) (inter.DataStore, error) {
	db, err := sql.Open("pgx", strings.TrimSpace(dsn))
	if err != nil {
		return nil, err
	}
	return openSQLStore(db, dialectPostgres)
}

func NewDataStorePostgres(dsn string) (inter.DataStore, error) {
	db, err := sql.Open("pgx", strings.TrimSpace(dsn))
	if err != nil {
		return nil, err
	}
	return newSQLStore(db, dialectPostgres)
}

func openSQLStore(db *sql.DB, dialect sqlDialect) (inter.DataStore, error) {
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if err := validateSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return &DataStoreSql{db: db, dialect: dialect}, nil
}

func newSQLStore(db *sql.DB, dialect sqlDialect) (inter.DataStore, error) {
	if err := initializeSchema(db, dialect); err != nil {
		db.Close()
		return nil, err
	}

	return openSQLStore(db, dialect)
}

// EnsureSQLiteSchema 显式初始化 SQLite 表结构。
func EnsureSQLiteSchema(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		return err
	}
	defer db.Close()
	return initializeSchema(db, dialectSQLite)
}

// EnsurePostgresSchema 显式初始化 PostgreSQL 表结构。
func EnsurePostgresSchema(dsn string) error {
	db, err := sql.Open("pgx", strings.TrimSpace(dsn))
	if err != nil {
		return err
	}
	defer db.Close()
	return initializeSchema(db, dialectPostgres)
}

func initializeSchema(db *sql.DB, dialect sqlDialect) error {
	if err := db.Ping(); err != nil {
		return err
	}

	if err := ensureBaseSchema(db, dialect); err != nil {
		return err
	}

	if err := runSchemaMigrations(db, dialect); err != nil {
		return err
	}
	return nil
}

func validateSchema(db *sql.DB) error {
	requiredTables := []string{
		"devices",
		"users",
		"tenants",
	}
	for _, table := range requiredTables {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			return fmt.Errorf("required table %s is unavailable: %w", table, err)
		}
	}
	return nil
}

func ensureBaseSchema(db *sql.DB, dialect sqlDialect) error {
	if dialect == dialectPostgres {
		return ensurePostgresBaseSchema(db)
	}
	return ensureSQLiteBaseSchema(db)
}

func ensureSQLiteBaseSchema(db *sql.DB) error {
	// 初始化原子化表结构
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
       created_at DATETIME,
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
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       uuid TEXT,
       tenant_id TEXT DEFAULT 'tenant_legacy',
       level TEXT,
       message TEXT,
       created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_logs_uuid ON logs (uuid);

    -- New Authboss-compatible Schema
    CREATE TABLE IF NOT EXISTS users (
       id            INTEGER PRIMARY KEY AUTOINCREMENT,
       email         TEXT,
       username      TEXT UNIQUE NOT NULL,
       password      TEXT NOT NULL,
       permission    INTEGER DEFAULT 0,

       oauth2_uid           TEXT,
       oauth2_provider      TEXT,
       oauth2_access_token  TEXT,
       oauth2_refresh_token TEXT,
       oauth2_expiry        DATETIME,
       remember_token       TEXT,

       recover_token        TEXT,
       recover_token_expiry DATETIME,

       confirm_token        TEXT,
       confirmed            BOOLEAN DEFAULT FALSE,

       last_login           DATETIME,
       created_at           DATETIME DEFAULT CURRENT_TIMESTAMP,
       updated_at           DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    -- External integration entities (e.g. Xiaomi via Home Assistant)
    CREATE TABLE IF NOT EXISTS integration_external_entities (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
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
       created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
       updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
       UNIQUE(source, entity_id)
    );
    CREATE INDEX IF NOT EXISTS idx_integration_entities_source_domain
      ON integration_external_entities (source, domain);
    CREATE INDEX IF NOT EXISTS idx_integration_entities_uuid
      ON integration_external_entities (goster_uuid);

    -- External observations that can carry numeric/bool/text/json values
    CREATE TABLE IF NOT EXISTS integration_external_observations (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
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
       created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
       UNIQUE(source, entity_id, ts, value_sig)
    );
    CREATE INDEX IF NOT EXISTS idx_integration_observations_query
      ON integration_external_observations (source, entity_id, ts);

    -- Optional command log for future control write-back
    CREATE TABLE IF NOT EXISTS integration_external_commands (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       tenant_id TEXT DEFAULT 'tenant_legacy',
       source TEXT NOT NULL,
       entity_id TEXT NOT NULL,
       command TEXT NOT NULL,
       payload_json TEXT,
       status TEXT NOT NULL DEFAULT 'pending',
       error_text TEXT,
       requested_at DATETIME DEFAULT CURRENT_TIMESTAMP,
       executed_at DATETIME
    );
    CREATE INDEX IF NOT EXISTS idx_integration_commands_status
      ON integration_external_commands (source, status, requested_at);

    CREATE TABLE IF NOT EXISTS tenants (
       id TEXT PRIMARY KEY,
       name TEXT NOT NULL UNIQUE,
       status TEXT NOT NULL DEFAULT 'active',
       created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
       updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS tenant_users (
       tenant_id TEXT NOT NULL,
       username TEXT NOT NULL,
       role TEXT NOT NULL,
       created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
       PRIMARY KEY (tenant_id, username),
       FOREIGN KEY (tenant_id) REFERENCES tenants(id)
    );
    CREATE INDEX IF NOT EXISTS idx_tenant_users_username ON tenant_users (username);

    CREATE TABLE IF NOT EXISTS device_groups (
       id TEXT PRIMARY KEY,
       tenant_id TEXT NOT NULL,
       name TEXT NOT NULL,
       description TEXT DEFAULT '',
       created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
       updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
       UNIQUE (tenant_id, name),
       FOREIGN KEY (tenant_id) REFERENCES tenants(id)
    );
    CREATE INDEX IF NOT EXISTS idx_device_groups_tenant ON device_groups (tenant_id);

    CREATE TABLE IF NOT EXISTS group_devices (
       group_id TEXT NOT NULL,
       device_uuid TEXT NOT NULL,
       created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
       PRIMARY KEY (group_id, device_uuid),
       FOREIGN KEY (group_id) REFERENCES device_groups(id)
    );
    CREATE INDEX IF NOT EXISTS idx_group_devices_uuid ON group_devices (device_uuid);

    CREATE TABLE IF NOT EXISTS group_users (
       group_id TEXT NOT NULL,
       username TEXT NOT NULL,
       role TEXT NOT NULL,
       created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
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

func ensurePostgresBaseSchema(db *sql.DB) error {
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
       created_at TIMESTAMPTZ,
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
       id BIGSERIAL PRIMARY KEY,
       uuid TEXT,
       tenant_id TEXT DEFAULT 'tenant_legacy',
       level TEXT,
       message TEXT,
       created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_logs_uuid ON logs (uuid);

    CREATE TABLE IF NOT EXISTS users (
       id BIGSERIAL PRIMARY KEY,
       email TEXT,
       username TEXT UNIQUE NOT NULL,
       password TEXT NOT NULL,
       permission INTEGER DEFAULT 0,

       oauth2_uid TEXT,
       oauth2_provider TEXT,
       oauth2_access_token TEXT,
       oauth2_refresh_token TEXT,
       oauth2_expiry TIMESTAMPTZ,
       remember_token TEXT,

       recover_token TEXT,
       recover_token_expiry TIMESTAMPTZ,

       confirm_token TEXT,
       confirmed BOOLEAN DEFAULT FALSE,

       last_login TIMESTAMPTZ,
       created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
       updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS integration_external_entities (
       id BIGSERIAL PRIMARY KEY,
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
       last_state_num DOUBLE PRECISION,
       last_state_bool INTEGER,
       last_seen_ts BIGINT,
       created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
       updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
       UNIQUE(source, entity_id)
    );
    CREATE INDEX IF NOT EXISTS idx_integration_entities_source_domain
      ON integration_external_entities (source, domain);
    CREATE INDEX IF NOT EXISTS idx_integration_entities_uuid
      ON integration_external_entities (goster_uuid);

    CREATE TABLE IF NOT EXISTS integration_external_observations (
       id BIGSERIAL PRIMARY KEY,
       tenant_id TEXT DEFAULT 'tenant_legacy',
       source TEXT NOT NULL,
       entity_id TEXT NOT NULL,
       ts BIGINT NOT NULL,
       value_num DOUBLE PRECISION,
       value_text TEXT,
       value_bool INTEGER,
       value_json TEXT,
       unit TEXT,
       value_sig TEXT NOT NULL DEFAULT '',
       raw_event_json TEXT,
       created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
       UNIQUE(source, entity_id, ts, value_sig)
    );
    CREATE INDEX IF NOT EXISTS idx_integration_observations_query
      ON integration_external_observations (source, entity_id, ts);

    CREATE TABLE IF NOT EXISTS integration_external_commands (
       id BIGSERIAL PRIMARY KEY,
       tenant_id TEXT DEFAULT 'tenant_legacy',
       source TEXT NOT NULL,
       entity_id TEXT NOT NULL,
       command TEXT NOT NULL,
       payload_json TEXT,
       status TEXT NOT NULL DEFAULT 'pending',
       error_text TEXT,
       requested_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
       executed_at TIMESTAMPTZ
    );
    CREATE INDEX IF NOT EXISTS idx_integration_commands_status
      ON integration_external_commands (source, status, requested_at);

    CREATE TABLE IF NOT EXISTS tenants (
       id TEXT PRIMARY KEY,
       name TEXT NOT NULL UNIQUE,
       status TEXT NOT NULL DEFAULT 'active',
       created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
       updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS tenant_users (
       tenant_id TEXT NOT NULL,
       username TEXT NOT NULL,
       role TEXT NOT NULL,
       created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
       PRIMARY KEY (tenant_id, username)
    );
    CREATE INDEX IF NOT EXISTS idx_tenant_users_username ON tenant_users (username);

    CREATE TABLE IF NOT EXISTS device_groups (
       id TEXT PRIMARY KEY,
       tenant_id TEXT NOT NULL,
       name TEXT NOT NULL,
       description TEXT DEFAULT '',
       created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
       updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
       UNIQUE (tenant_id, name)
    );
    CREATE INDEX IF NOT EXISTS idx_device_groups_tenant ON device_groups (tenant_id);

    CREATE TABLE IF NOT EXISTS group_devices (
       group_id TEXT NOT NULL,
       device_uuid TEXT NOT NULL,
       created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
       PRIMARY KEY (group_id, device_uuid)
    );
    CREATE INDEX IF NOT EXISTS idx_group_devices_uuid ON group_devices (device_uuid);

    CREATE TABLE IF NOT EXISTS group_users (
       group_id TEXT NOT NULL,
       username TEXT NOT NULL,
       role TEXT NOT NULL,
       created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
       PRIMARY KEY (group_id, username)
    );
    CREATE INDEX IF NOT EXISTS idx_group_users_username ON group_users (username);
    `

	if _, err := db.Exec(schema); err != nil {
		return err
	}
	return nil
}

func runSchemaMigrations(db *sql.DB, dialect sqlDialect) error {
	if dialect == dialectPostgres {
		return runPostgresBootstrap(db)
	}
	return runSQLiteSchemaMigrations(db)
}

func runSQLiteSchemaMigrations(db *sql.DB) error {
	// 兼容旧库结构：为历史数据库补齐多租户字段。
	columnMigrations := []string{
		"ALTER TABLE metrics ADD COLUMN type INTEGER DEFAULT 0",
		"ALTER TABLE devices ADD COLUMN tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE metrics ADD COLUMN tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE logs ADD COLUMN tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE integration_external_entities ADD COLUMN tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE integration_external_observations ADD COLUMN tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE integration_external_commands ADD COLUMN tenant_id TEXT DEFAULT 'tenant_legacy'",
	}
	for _, stmt := range columnMigrations {
		if _, err := db.Exec(stmt); err != nil && !isDuplicateColumnError(err) {
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
		"INSERT INTO tenants (id, name, status) VALUES ('tenant_legacy', 'legacy', 'active') ON CONFLICT(id) DO NOTHING",
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
		  ON CONFLICT(tenant_id, username) DO UPDATE SET role = excluded.role`,
	}
	for _, stmt := range postMigrations {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func runPostgresBootstrap(db *sql.DB) error {
	postBootstrap := []string{
		"CREATE INDEX IF NOT EXISTS idx_metrics_type ON metrics (uuid, type, ts)",
		"CREATE INDEX IF NOT EXISTS idx_devices_tenant_uuid ON devices (tenant_id, uuid)",
		"CREATE INDEX IF NOT EXISTS idx_metrics_tenant_uuid_ts ON metrics (tenant_id, uuid, ts)",
		"CREATE INDEX IF NOT EXISTS idx_logs_tenant_uuid_created ON logs (tenant_id, uuid, created_at)",
		"CREATE INDEX IF NOT EXISTS idx_ext_entities_tenant_source_domain ON integration_external_entities (tenant_id, source, domain)",
		"CREATE INDEX IF NOT EXISTS idx_ext_obs_tenant_source_entity_ts ON integration_external_observations (tenant_id, source, entity_id, ts)",
		"CREATE INDEX IF NOT EXISTS idx_ext_cmd_tenant_source_status ON integration_external_commands (tenant_id, source, status, requested_at)",
		"INSERT INTO tenants (id, name, status) VALUES ('tenant_legacy', 'legacy', 'active') ON CONFLICT(id) DO NOTHING",
	}
	for _, stmt := range postBootstrap {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("postgres bootstrap failed: %w", err)
		}
	}
	return nil
}

func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column name")
}
