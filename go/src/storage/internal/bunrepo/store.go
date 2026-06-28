package bunrepo

import (
	"database/sql"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/schema"
	_ "modernc.org/sqlite"
)

// Store 持有 Bun DB 连接，供各个存储子模块共享。
type Store struct {
	DB *bun.DB
}

func OpenSQLite(path string) (*Store, error) {
	sqlDB, err := sql.Open("sqlite", path+"?_loc=Local")
	if err != nil {
		return nil, err
	}
	return open(sqlDB, sqlitedialect.New())
}

func OpenPostgres(dsn string) (*Store, error) {
	sqlDB, err := sql.Open("pgx", strings.TrimSpace(dsn))
	if err != nil {
		return nil, err
	}
	return open(sqlDB, pgdialect.New())
}

func open(sqlDB *sql.DB, dialect schema.Dialect) (*Store, error) {
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return &Store{DB: bun.NewDB(sqlDB, dialect)}, nil
}

func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}
