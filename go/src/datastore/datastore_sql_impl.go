package datastore

import (
	"database/sql"

	"github.com/nhirsama/Goster-IoT/src/inter"
	_ "modernc.org/sqlite"
)

type DataStoreSql struct {
	db *sql.DB
}

func NewDataStoreSql(dbPath string) (inter.DataStore, error) {
	// 强制指定 _loc=Local，确保 DATETIME 字段按本地时区读写，防止时区转换偏移
	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
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
       value REAL,
       type INTEGER DEFAULT 0
    );
    CREATE INDEX IF NOT EXISTS idx_metrics_query ON metrics (uuid, ts);
    CREATE INDEX IF NOT EXISTS idx_metrics_type ON metrics (uuid, type, ts);

    CREATE TABLE IF NOT EXISTS logs (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       uuid TEXT,
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
    `

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	// Migration: Ensure 'type' column exists for existing databases.
	// SQLite allows adding columns. Ignore error if column already exists (duplicate column name).
	db.Exec("ALTER TABLE metrics ADD COLUMN type INTEGER DEFAULT 0")

	return &DataStoreSql{db: db}, nil
}
