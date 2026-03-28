package dbschema

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestEnsureSQLiteAppliesMigrationsAndRecordsVersion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "schema.db")
	if err := EnsureSQLite(dbPath); err != nil {
		t.Fatalf("EnsureSQLite failed: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer db.Close()

	var version string
	if err := db.QueryRow(`SELECT version FROM schema_migrations ORDER BY version ASC LIMIT 1`).Scan(&version); err != nil {
		t.Fatalf("read schema_migrations failed: %v", err)
	}
	if version != "0001_init.sql" {
		t.Fatalf("unexpected migration version: %s", version)
	}
}

func TestEnsureSQLiteIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "schema.db")
	if err := EnsureSQLite(dbPath); err != nil {
		t.Fatalf("first EnsureSQLite failed: %v", err)
	}
	if err := EnsureSQLite(dbPath); err != nil {
		t.Fatalf("second EnsureSQLite failed: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count schema_migrations failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", count)
	}
}

func TestEnsureSQLiteMigratesLegacyDeviceCommands(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "schema_legacy_device_commands.db")
	if err := EnsureSQLite(dbPath); err != nil {
		t.Fatalf("initial EnsureSQLite failed: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = '0002_device_commands.sql'`); err != nil {
		t.Fatalf("delete migration version failed: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE device_commands`); err != nil {
		t.Fatalf("drop device_commands failed: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO integration_external_commands
		    (id, tenant_id, source, entity_id, command, payload_json, status, error_text)
		VALUES
		    (17, 'tenant_demo', 'goster_device', 'device-legacy', 'action_exec:3', '{"relay":"on"}', 'sent', NULL)
	`); err != nil {
		t.Fatalf("seed legacy device command failed: %v", err)
	}

	if err := EnsureSQLite(dbPath); err != nil {
		t.Fatalf("second EnsureSQLite failed: %v", err)
	}

	var row struct {
		TenantID string
		UUID     string
		CmdID    int
		Command  string
		Status   string
	}
	if err := db.QueryRow(`
		SELECT tenant_id, uuid, cmd_id, command, status
		FROM device_commands
		WHERE id = 17
	`).Scan(&row.TenantID, &row.UUID, &row.CmdID, &row.Command, &row.Status); err != nil {
		t.Fatalf("query migrated device command failed: %v", err)
	}
	if row.TenantID != "tenant_demo" || row.UUID != "device-legacy" || row.CmdID != 3 || row.Command != "action_exec" || row.Status != "sent" {
		t.Fatalf("unexpected migrated device command row: %+v", row)
	}

	var legacyCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM integration_external_commands WHERE source = 'goster_device'`).Scan(&legacyCount); err != nil {
		t.Fatalf("count legacy integration_external_commands failed: %v", err)
	}
	if legacyCount != 0 {
		t.Fatalf("expected legacy goster_device rows to be removed, got %d", legacyCount)
	}
}

func TestSplitSQLStatements(t *testing.T) {
	sqlText := "INSERT INTO demo VALUES ('a;b');\nCREATE TABLE demo (id INTEGER);\n"
	stmts := splitSQLStatements(sqlText)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
}
