package datastore

import (
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
)

func TestNewDataStoreSqlCreatesMultiTenantSchema(t *testing.T) {
	t.Skip("Skipping migration test - requires PostgreSQL test database")
}

func TestNewDataStoreSqlMigratesLegacySchema(t *testing.T) {
	t.Skip("Skipping migration test - requires PostgreSQL test database")
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()

	var name string
	err := db.QueryRow(`SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename = $1`, table).Scan(&name)
	return err == nil && name == table
}

func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()

	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public'
			AND table_name = $1
			AND column_name = $2
		)
	`, table, column).Scan(&exists)
	if err != nil {
		t.Fatalf("query column existence failed: %v", err)
	}
	return exists
}
