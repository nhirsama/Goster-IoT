package datastore

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewDataStoreSqlCreatesMultiTenantSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "new_multitenant.db")
	store, err := NewDataStoreSql(dbPath)
	if err != nil {
		t.Fatalf("NewDataStoreSql failed: %v", err)
	}
	sqlStore := asSQLStore(t, store)

	requiredTables := []string{
		"tenants",
		"tenant_users",
		"device_groups",
		"group_devices",
		"group_users",
	}
	for _, table := range requiredTables {
		if !tableExists(t, sqlStore.db, table) {
			t.Fatalf("table %s should exist", table)
		}
	}

	columnChecks := map[string][]string{
		"devices":                           {"tenant_id"},
		"metrics":                           {"tenant_id", "type"},
		"logs":                              {"tenant_id"},
		"integration_external_entities":     {"tenant_id"},
		"integration_external_observations": {"tenant_id"},
		"integration_external_commands":     {"tenant_id"},
	}
	for table, columns := range columnChecks {
		for _, col := range columns {
			if !columnExists(t, sqlStore.db, table, col) {
				t.Fatalf("column %s.%s should exist", table, col)
			}
		}
	}

	var tenantName, tenantStatus string
	if err := sqlStore.db.QueryRow("SELECT name, status FROM tenants WHERE id = ?", "tenant_legacy").Scan(&tenantName, &tenantStatus); err != nil {
		t.Fatalf("legacy tenant should exist: %v", err)
	}
	if tenantName != "legacy" || tenantStatus != "active" {
		t.Fatalf("unexpected legacy tenant: name=%s status=%s", tenantName, tenantStatus)
	}
}

func TestNewDataStoreSqlMigratesLegacySchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy_schema.db")
	raw, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		t.Fatalf("open legacy db failed: %v", err)
	}

	legacySchema := `
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

	CREATE TABLE IF NOT EXISTS logs (
	   id INTEGER PRIMARY KEY AUTOINCREMENT,
	   uuid TEXT,
	   level TEXT,
	   message TEXT,
	   created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS users (
	   id INTEGER PRIMARY KEY AUTOINCREMENT,
	   email TEXT,
	   username TEXT UNIQUE NOT NULL,
	   password TEXT NOT NULL,
	   permission INTEGER DEFAULT 0
	);

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
	`
	if _, err := raw.Exec(legacySchema); err != nil {
		raw.Close()
		t.Fatalf("create legacy schema failed: %v", err)
	}

	if _, err := raw.Exec(`
		INSERT INTO devices (uuid, name, hw_version, sw_version, config_version, sn, mac, created_at, token, auth_status)
		VALUES ('dev-1', 'Legacy Device', 'hw1', 'sw1', 'cfg1', 'sn1', 'mac1', CURRENT_TIMESTAMP, 'tk-1', 0)
	`); err != nil {
		raw.Close()
		t.Fatalf("seed legacy device failed: %v", err)
	}
	if _, err := raw.Exec("INSERT INTO metrics (uuid, ts, value) VALUES ('dev-1', 1700000000000, 10.5)"); err != nil {
		raw.Close()
		t.Fatalf("seed legacy metrics failed: %v", err)
	}
	if _, err := raw.Exec("INSERT INTO logs (uuid, level, message) VALUES ('dev-1', 'INFO', 'legacy log')"); err != nil {
		raw.Close()
		t.Fatalf("seed legacy logs failed: %v", err)
	}
	if _, err := raw.Exec(`
		INSERT INTO integration_external_commands (source, entity_id, command, payload_json, status)
		VALUES ('goster_device', 'dev-1', 'action_exec:515', '{"op":"reboot"}', 'queued')
	`); err != nil {
		raw.Close()
		t.Fatalf("seed legacy command failed: %v", err)
	}
	if _, err := raw.Exec(`
		INSERT INTO users (email, username, password, permission)
		VALUES ('admin@test.local', 'admin', 'pw', 3), ('viewer@test.local', 'viewer', 'pw', 1)
	`); err != nil {
		raw.Close()
		t.Fatalf("seed legacy users failed: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close legacy db failed: %v", err)
	}

	store, err := NewDataStoreSql(dbPath)
	if err != nil {
		t.Fatalf("migrate legacy db failed: %v", err)
	}
	sqlStore := asSQLStore(t, store)

	columnChecks := map[string][]string{
		"devices":                           {"tenant_id"},
		"metrics":                           {"tenant_id", "type"},
		"logs":                              {"tenant_id"},
		"integration_external_entities":     {"tenant_id"},
		"integration_external_observations": {"tenant_id"},
		"integration_external_commands":     {"tenant_id"},
	}
	for table, columns := range columnChecks {
		for _, col := range columns {
			if !columnExists(t, sqlStore.db, table, col) {
				t.Fatalf("legacy migration should add %s.%s", table, col)
			}
		}
	}

	checkLegacyTenantBackfill := func(table string) {
		t.Helper()
		var count int
		query := "SELECT COUNT(*) FROM " + table + " WHERE tenant_id = 'tenant_legacy'"
		if err := sqlStore.db.QueryRow(query).Scan(&count); err != nil {
			t.Fatalf("query %s tenant backfill failed: %v", table, err)
		}
		if count == 0 {
			t.Fatalf("table %s should have legacy tenant backfilled rows", table)
		}
	}
	checkLegacyTenantBackfill("devices")
	checkLegacyTenantBackfill("metrics")
	checkLegacyTenantBackfill("logs")
	checkLegacyTenantBackfill("integration_external_commands")

	var adminRole string
	if err := sqlStore.db.QueryRow(`SELECT role FROM tenant_users WHERE tenant_id = 'tenant_legacy' AND username = 'admin'`).Scan(&adminRole); err != nil {
		t.Fatalf("admin tenant role row missing: %v", err)
	}
	if adminRole != "tenant_admin" {
		t.Fatalf("admin role mismatch: got=%s want=tenant_admin", adminRole)
	}

	var viewerRole string
	if err := sqlStore.db.QueryRow(`SELECT role FROM tenant_users WHERE tenant_id = 'tenant_legacy' AND username = 'viewer'`).Scan(&viewerRole); err != nil {
		t.Fatalf("viewer tenant role row missing: %v", err)
	}
	if viewerRole != "tenant_ro" {
		t.Fatalf("viewer role mismatch: got=%s want=tenant_ro", viewerRole)
	}
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()

	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, table).Scan(&name)
	return err == nil && name == table
}

func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()

	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("PRAGMA table_info(%s) failed: %v", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info(%s) failed: %v", table, err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table_info(%s) failed: %v", table, err)
	}
	return false
}
