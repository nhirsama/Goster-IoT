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
	if count != 1 {
		t.Fatalf("expected 1 applied migration, got %d", count)
	}
}

func TestSplitSQLStatements(t *testing.T) {
	sqlText := "INSERT INTO demo VALUES ('a;b');\nCREATE TABLE demo (id INTEGER);\n"
	stmts := splitSQLStatements(sqlText)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
}
