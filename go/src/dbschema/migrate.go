package dbschema

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

const migrationsTable = "schema_migrations"

// EnsureSQLite 根据 go/db/migrations/sqlite 中的迁移文件初始化数据库。
func EnsureSQLite(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		return err
	}
	defer db.Close()
	return Ensure(db, "sqlite")
}

// EnsurePostgres 根据 go/db/migrations/postgres 中的迁移文件初始化数据库。
func EnsurePostgres(dsn string) error {
	db, err := sql.Open("pgx", strings.TrimSpace(dsn))
	if err != nil {
		return err
	}
	defer db.Close()
	return Ensure(db, "postgres")
}

// Ensure 按驱动执行尚未应用的迁移文件。
func Ensure(db *sql.DB, driver string) error {
	if db == nil {
		return fmt.Errorf("migration db is required")
	}
	driver = normalizeDriver(driver)
	if driver == "" {
		return fmt.Errorf("unsupported migration driver")
	}
	if err := db.Ping(); err != nil {
		return err
	}
	if err := ensureMigrationsTable(db); err != nil {
		return err
	}

	applied, err := loadAppliedVersions(db)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		legacy, err := looksLikeLegacySchema(db)
		if err != nil {
			return err
		}
		if legacy {
			if err := applyCompatColumnUpgrades(db, driver); err != nil {
				return err
			}
		}
	}
	files, err := listMigrationFiles(driver)
	if err != nil {
		return err
	}
	for _, file := range files {
		version := filepath.Base(file)
		if _, ok := applied[version]; ok {
			continue
		}
		if err := applyMigrationFile(db, version, file); err != nil {
			return err
		}
	}
	return applyCompatUpgrades(db, driver)
}

func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
)`)
	return err
}

func loadAppliedVersions(db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]struct{})
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		out[strings.TrimSpace(version)] = struct{}{}
	}
	return out, rows.Err()
}

func listMigrationFiles(driver string) ([]string, error) {
	dir, err := migrationDir(driver)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".sql") {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	sort.Strings(files)
	return files, nil
}

func applyMigrationFile(db *sql.DB, version, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	statements := splitSQLStatements(string(content))
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("apply migration %s failed: %w", version, err)
		}
	}
	if _, err := tx.Exec(
		"INSERT INTO "+migrationsTable+"(version) VALUES ("+quoteSQLLiteral(version)+")",
	); err != nil {
		return fmt.Errorf("record migration %s failed: %w", version, err)
	}
	return tx.Commit()
}

func splitSQLStatements(sqlText string) []string {
	var (
		out       []string
		builder   strings.Builder
		inSingle  bool
		inDouble  bool
		prevSlash bool
	)

	for _, r := range sqlText {
		switch r {
		case '\'':
			if !inDouble && !prevSlash {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle && !prevSlash {
				inDouble = !inDouble
			}
		case ';':
			if !inSingle && !inDouble {
				stmt := strings.TrimSpace(builder.String())
				if stmt != "" {
					out = append(out, stmt)
				}
				builder.Reset()
				prevSlash = false
				continue
			}
		}

		builder.WriteRune(r)
		prevSlash = r == '\\'
	}

	stmt := strings.TrimSpace(builder.String())
	if stmt != "" {
		out = append(out, stmt)
	}
	return out
}

func migrationDir(driver string) (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve migration path failed")
	}
	base := filepath.Dir(file)
	dir := filepath.Clean(filepath.Join(base, "..", "..", "db", "migrations", driver))
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("migration dir not found: %s", dir)
	}
	return dir, nil
}

func normalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "sqlite":
		return "sqlite"
	case "postgres":
		return "postgres"
	default:
		return ""
	}
}

func quoteSQLLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func applyCompatUpgrades(db *sql.DB, driver string) error {
	switch driver {
	case "sqlite":
		if err := applySQLiteCompatColumns(db); err != nil {
			return err
		}
		return applySQLiteCompatPost(db)
	case "postgres":
		if err := applyPostgresCompatColumns(db); err != nil {
			return err
		}
		return applyPostgresCompatPost(db)
	default:
		return fmt.Errorf("unsupported migration driver: %s", driver)
	}
}

func applyCompatColumnUpgrades(db *sql.DB, driver string) error {
	switch driver {
	case "sqlite":
		return applySQLiteCompatColumns(db)
	case "postgres":
		return applyPostgresCompatColumns(db)
	default:
		return fmt.Errorf("unsupported migration driver: %s", driver)
	}
}

func looksLikeLegacySchema(db *sql.DB) (bool, error) {
	for _, table := range []string{"devices", "metrics", "logs", "users"} {
		ok, err := tableExists(db, table)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func tableExists(db *sql.DB, table string) (bool, error) {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "no such table") || strings.Contains(msg, "does not exist") || strings.Contains(msg, "undefined table") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func applySQLiteCompatColumns(db *sql.DB) error {
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
		if _, err := db.Exec(stmt); err != nil && !isSQLiteDuplicateColumnError(err) {
			return err
		}
	}
	return nil
}

func applySQLiteCompatPost(db *sql.DB) error {
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

func applyPostgresCompatColumns(db *sql.DB) error {
	stmts := []string{
		"ALTER TABLE devices ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE metrics ADD COLUMN IF NOT EXISTS type INTEGER DEFAULT 0",
		"ALTER TABLE metrics ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE logs ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE integration_external_entities ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE integration_external_observations ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'tenant_legacy'",
		"ALTER TABLE integration_external_commands ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'tenant_legacy'",
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("postgres compat upgrade failed: %w", err)
		}
	}
	return nil
}

func applyPostgresCompatPost(db *sql.DB) error {
	stmts := []string{
		"CREATE INDEX IF NOT EXISTS idx_metrics_type ON metrics (uuid, type, ts)",
		"CREATE INDEX IF NOT EXISTS idx_devices_tenant_uuid ON devices (tenant_id, uuid)",
		"CREATE INDEX IF NOT EXISTS idx_metrics_tenant_uuid_ts ON metrics (tenant_id, uuid, ts)",
		"CREATE INDEX IF NOT EXISTS idx_logs_tenant_uuid_created ON logs (tenant_id, uuid, created_at)",
		"CREATE INDEX IF NOT EXISTS idx_ext_entities_tenant_source_domain ON integration_external_entities (tenant_id, source, domain)",
		"CREATE INDEX IF NOT EXISTS idx_ext_obs_tenant_source_entity_ts ON integration_external_observations (tenant_id, source, entity_id, ts)",
		"CREATE INDEX IF NOT EXISTS idx_ext_cmd_tenant_source_status ON integration_external_commands (tenant_id, source, status, requested_at)",
		"INSERT INTO tenants (id, name, status) VALUES ('tenant_legacy', 'legacy', 'active') ON CONFLICT(id) DO NOTHING",
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("postgres compat upgrade failed: %w", err)
		}
	}
	return nil
}

func isSQLiteDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
}
