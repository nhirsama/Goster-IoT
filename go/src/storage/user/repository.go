package user

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/bunrepo"
	"github.com/uptrace/bun"
)

type Repository struct {
	db *bun.DB
}

func NewRepository(db *bun.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserCount() (int, error) {
	var count int
	err := r.db.NewSelect().
		Table("users").
		ColumnExpr("COUNT(*)").
		Scan(context.Background(), &count)
	return count, err
}

func (r *Repository) ListUsers() ([]inter.User, error) {
	var rows []bunrepo.UserRow
	if err := r.db.NewSelect().
		Model(&rows).
		Column("username", "permission", "created_at").
		OrderExpr("created_at ASC, id ASC").
		Scan(context.Background()); err != nil {
		return nil, err
	}

	out := make([]inter.User, 0, len(rows))
	for _, row := range rows {
		user := inter.User{
			Permission: inter.PermissionType(row.Permission),
			CreatedAt:  row.CreatedAt,
		}
		if row.Username.Valid {
			user.Username = row.Username.String
		} else {
			user.Username = "Unknown"
		}
		out = append(out, user)
	}
	return out, nil
}

func (r *Repository) GetUserPermission(username string) (inter.PermissionType, error) {
	var row struct {
		Permission int `bun:"permission"`
	}
	err := r.db.NewSelect().
		Table("users").
		Column("permission").
		Where("username = ?", username).
		Limit(1).
		Scan(context.Background(), &row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return inter.PermissionNone, errors.New("user not found")
		}
		return inter.PermissionNone, err
	}
	return inter.PermissionType(row.Permission), nil
}

func (r *Repository) UpdateUserPermission(username string, perm inter.PermissionType) error {
	username = strings.TrimSpace(username)
	return r.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewUpdate().
			Table("users").
			Set("permission = ?", int(perm)).
			Where("username = ?", username).
			Returning("NULL").
			Exec(ctx)
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
		return SyncLegacyTenantRole(ctx, tx, username, perm)
	})
}

func SyncLegacyTenantRole(ctx context.Context, db bun.IDB, username string, perm inter.PermissionType) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil
	}
	if _, err := db.NewRaw(
		"INSERT INTO tenants (id, name, status) VALUES (?, ?, 'active') ON CONFLICT(id) DO NOTHING",
		bunrepo.LegacyTenantID,
		"legacy",
	).Exec(ctx); err != nil {
		return err
	}
	_, err := db.NewRaw(`
		INSERT INTO tenant_users (tenant_id, username, role)
		VALUES (?, ?, ?)
		ON CONFLICT(tenant_id, username) DO UPDATE SET role = excluded.role
	`, bunrepo.LegacyTenantID, username, string(bunrepo.PermissionToTenantRole(perm))).Exec(ctx)
	return err
}
