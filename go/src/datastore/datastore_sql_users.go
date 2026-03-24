package datastore

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// GetUserCount 获取注册用户总数
func (s *DataStoreSql) GetUserCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// ListUsers 获取所有用户列表
func (s *DataStoreSql) ListUsers() ([]inter.User, error) {
	rows, err := s.db.Query("SELECT username, permission, created_at FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []inter.User
	for rows.Next() {
		var u inter.User
		var perm int
		var username sql.NullString
		// username is nullable in new schema
		if err := rows.Scan(&username, &perm, &u.CreatedAt); err != nil {
			return nil, err
		}
		if username.Valid {
			u.Username = username.String
		} else {
			u.Username = "Unknown"
		}
		u.Permission = inter.PermissionType(perm)
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

// GetUserPermission 获取指定用户的当前权限 (Using username for legacy support in UI)
func (s *DataStoreSql) GetUserPermission(username string) (inter.PermissionType, error) {
	var perm int
	err := s.db.QueryRow("SELECT permission FROM users WHERE username = ?", username).Scan(&perm)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return inter.PermissionNone, errors.New("user not found")
		}
		return inter.PermissionNone, err
	}
	return inter.PermissionType(perm), nil
}

// UpdateUserPermission 更新用户权限
func (s *DataStoreSql) UpdateUserPermission(username string, perm inter.PermissionType) error {
	username = strings.TrimSpace(username)
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec("UPDATE users SET permission = ? WHERE username = ?", perm, username)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("user not found")
	}
	if err := s.syncLegacyTenantRoleTx(context.Background(), tx, username, perm); err != nil {
		return err
	}
	return tx.Commit()
}
