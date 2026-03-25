package device

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

func (r *Repository) InitDevice(uuid string, meta inter.DeviceMetadata) error {
	_, err := r.db.NewInsert().
		Model(bunrepo.NewDeviceModel(uuid, meta)).
		Returning("NULL").
		Exec(context.Background())
	return err
}

func (r *Repository) DestroyDevice(uuid string) error {
	return r.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewDelete().
			Model((*bunrepo.DeviceModel)(nil)).
			Where("uuid = ?", uuid).
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
			return errors.New("device not found")
		}
		if _, err := tx.NewRaw("DELETE FROM metrics WHERE uuid = ?", uuid).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewRaw("DELETE FROM logs WHERE uuid = ?", uuid).Exec(ctx); err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) LoadConfig(uuid string) (inter.DeviceMetadata, error) {
	var row bunrepo.DeviceModel
	err := r.db.NewSelect().
		Model(&row).
		Where("uuid = ?", uuid).
		Limit(1).
		Scan(context.Background())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return inter.DeviceMetadata{}, errors.New("device not found")
		}
		return inter.DeviceMetadata{}, err
	}
	return row.ToMetadata(), nil
}

func (r *Repository) SaveMetadata(uuid string, meta inter.DeviceMetadata) error {
	_, err := r.db.NewUpdate().
		Model((*bunrepo.DeviceModel)(nil)).
		Set("name = ?", meta.Name).
		Set("hw_version = ?", meta.HWVersion).
		Set("sw_version = ?", meta.SWVersion).
		Set("config_version = ?", meta.ConfigVersion).
		Set("sn = ?", meta.SerialNumber).
		Set("mac = ?", meta.MACAddress).
		Set("auth_status = ?", int(meta.AuthenticateStatus)).
		Set("token = ?", bunrepo.NullableToken(meta.Token)).
		Where("uuid = ?", uuid).
		Returning("NULL").
		Exec(context.Background())
	return err
}

func (r *Repository) ListDevices(page, size int) ([]inter.DeviceRecord, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 100
	}

	var rows []bunrepo.DeviceModel
	err := r.db.NewSelect().
		Model(&rows).
		OrderExpr("created_at ASC, uuid ASC").
		Limit(size).
		Offset((page - 1) * size).
		Scan(context.Background())
	if err != nil {
		return nil, err
	}

	out := make([]inter.DeviceRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ToRecord())
	}
	return out, nil
}

func (r *Repository) ListDevicesByStatus(status inter.AuthenticateStatusType, page, size int) ([]inter.DeviceRecord, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 100
	}

	var rows []bunrepo.DeviceModel
	err := r.db.NewSelect().
		Model(&rows).
		Where("auth_status = ?", int(status)).
		OrderExpr("created_at ASC, uuid ASC").
		Limit(size).
		Offset((page - 1) * size).
		Scan(context.Background())
	if err != nil {
		return nil, err
	}

	out := make([]inter.DeviceRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ToRecord())
	}
	return out, nil
}

func (r *Repository) GetDeviceByToken(token string) (string, inter.AuthenticateStatusType, error) {
	var row struct {
		UUID       string `bun:"uuid"`
		AuthStatus int    `bun:"auth_status"`
	}
	err := r.db.NewSelect().
		Table("devices").
		Column("uuid", "auth_status").
		Where("token = ?", token).
		Limit(1).
		Scan(context.Background(), &row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", 0, fmt.Errorf("token not found: %w", err)
		}
		return "", 0, err
	}
	return row.UUID, inter.AuthenticateStatusType(row.AuthStatus), nil
}

func (r *Repository) UpdateToken(uuid string, newToken string) error {
	res, err := r.db.NewUpdate().
		Model((*bunrepo.DeviceModel)(nil)).
		Set("token = ?", bunrepo.NullableToken(newToken)).
		Where("uuid = ?", uuid).
		Returning("NULL").
		Exec(context.Background())
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("device not found")
	}
	return nil
}

func (r *Repository) ResolveDeviceTenant(uuid string) (string, error) {
	var row struct {
		TenantID sql.NullString `bun:"tenant_id"`
	}
	err := r.db.NewSelect().
		Table("devices").
		Column("tenant_id").
		Where("uuid = ?", uuid).
		Limit(1).
		Scan(context.Background(), &row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("device not found")
		}
		return "", err
	}
	if row.TenantID.Valid && strings.TrimSpace(row.TenantID.String) != "" {
		return strings.TrimSpace(row.TenantID.String), nil
	}
	return bunrepo.DefaultTenantID, nil
}

func (r *Repository) LoadConfigByTenant(tenantID, uuid string) (inter.DeviceMetadata, error) {
	var row bunrepo.DeviceModel
	err := r.db.NewSelect().
		Model(&row).
		Where("uuid = ?", uuid).
		Where("tenant_id = ?", bunrepo.NormalizeTenantID(tenantID)).
		Limit(1).
		Scan(context.Background())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return inter.DeviceMetadata{}, errors.New("device not found")
		}
		return inter.DeviceMetadata{}, err
	}
	return row.ToMetadata(), nil
}

func (r *Repository) ListDevicesByTenant(tenantID string, status *inter.AuthenticateStatusType, page, size int) ([]inter.DeviceRecord, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 100
	}

	query := r.db.NewSelect().
		Model((*bunrepo.DeviceModel)(nil)).
		Where("tenant_id = ?", bunrepo.NormalizeTenantID(tenantID)).
		OrderExpr("created_at ASC, uuid ASC").
		Limit(size).
		Offset((page - 1) * size)
	if status != nil {
		query = query.Where("auth_status = ?", int(*status))
	}

	var rows []bunrepo.DeviceModel
	if err := query.Scan(context.Background(), &rows); err != nil {
		return nil, err
	}

	out := make([]inter.DeviceRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ToRecord())
	}
	return out, nil
}
