package datastore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type sqlDialect string

const (
	dialectSQLite   sqlDialect = "sqlite"
	dialectPostgres sqlDialect = "postgres"
)

func (s *DataStoreSql) rebind(query string) string {
	if s == nil || s.dialect != dialectPostgres {
		return query
	}

	var b strings.Builder
	b.Grow(len(query) + 8)
	argIndex := 1
	for _, ch := range query {
		if ch == '?' {
			b.WriteString(fmt.Sprintf("$%d", argIndex))
			argIndex++
			continue
		}
		b.WriteRune(ch)
	}
	return b.String()
}

func (s *DataStoreSql) exec(query string, args ...interface{}) (sql.Result, error) {
	return s.db.Exec(s.rebind(query), args...)
}

func (s *DataStoreSql) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.db.ExecContext(ctx, s.rebind(query), args...)
}

func (s *DataStoreSql) query(query string, args ...interface{}) (*sql.Rows, error) {
	return s.db.Query(s.rebind(query), args...)
}

func (s *DataStoreSql) queryRow(query string, args ...interface{}) *sql.Row {
	return s.db.QueryRow(s.rebind(query), args...)
}

func (s *DataStoreSql) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return s.db.QueryRowContext(ctx, s.rebind(query), args...)
}

func (s *DataStoreSql) begin() (*sql.Tx, error) {
	return s.db.Begin()
}

func (s *DataStoreSql) beginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, opts)
}

func (s *DataStoreSql) execTx(tx *sql.Tx, query string, args ...interface{}) (sql.Result, error) {
	return tx.Exec(s.rebind(query), args...)
}

func (s *DataStoreSql) execTxContext(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) (sql.Result, error) {
	return tx.ExecContext(ctx, s.rebind(query), args...)
}

func (s *DataStoreSql) queryRowTxContext(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) *sql.Row {
	return tx.QueryRowContext(ctx, s.rebind(query), args...)
}

func (s *DataStoreSql) prepareTx(tx *sql.Tx, query string) (*sql.Stmt, error) {
	return tx.Prepare(s.rebind(query))
}

func (s *DataStoreSql) insertReturningID(query string, args ...interface{}) (int64, error) {
	var id int64
	if err := s.queryRow(query, args...).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *DataStoreSql) insertReturningIDContext(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) (int64, error) {
	var id int64
	if err := s.queryRowTxContext(ctx, tx, query, args...).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "unique violation")
}
