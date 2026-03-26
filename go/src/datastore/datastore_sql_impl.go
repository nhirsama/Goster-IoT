package datastore

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nhirsama/Goster-IoT/src/dbschema"
	"github.com/nhirsama/Goster-IoT/src/inter"
	_ "modernc.org/sqlite"
)

type DataStoreSql struct {
	db      *sql.DB
	dialect sqlDialect
}

func (ds *DataStoreSql) Close() error {
	if ds == nil || ds.db == nil {
		return nil
	}
	return ds.db.Close()
}

// OpenDataStoreSql 只打开现有的 SQLite 数据库，不隐式建表。
func OpenDataStoreSql(dbPath string) (inter.DataStore, error) {
	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		return nil, err
	}
	return openSQLStore(db, dialectSQLite)
}

// NewDataStoreSql 保留 legacy bootstrap 入口。
// 实际 schema 初始化已经统一迁移到 go/db 资产目录。
func NewDataStoreSql(dbPath string) (inter.DataStore, error) {
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

// NewDataStorePostgres 保留 legacy bootstrap 入口。
// 实际 schema 初始化已经统一迁移到 go/db 资产目录。
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
	driver, err := migrationDriverName(dialect)
	if err != nil {
		db.Close()
		return nil, err
	}
	if err := dbschema.Ensure(db, driver); err != nil {
		db.Close()
		return nil, err
	}
	return openSQLStore(db, dialect)
}

// EnsureSQLiteSchema 显式初始化 SQLite 表结构。
func EnsureSQLiteSchema(dbPath string) error {
	return dbschema.EnsureSQLite(dbPath)
}

// EnsurePostgresSchema 显式初始化 PostgreSQL 表结构。
func EnsurePostgresSchema(dsn string) error {
	return dbschema.EnsurePostgres(dsn)
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

func migrationDriverName(dialect sqlDialect) (string, error) {
	switch dialect {
	case dialectSQLite:
		return "sqlite", nil
	case dialectPostgres:
		return "postgres", nil
	default:
		return "", fmt.Errorf("unsupported dialect: %s", dialect)
	}
}
