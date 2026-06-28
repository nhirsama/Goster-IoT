package telemetry

import (
	"context"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/storage/device"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/bunrepo"
	"github.com/uptrace/bun"
)

type Repository struct {
	db         *bun.DB
	deviceRepo interface {
		ResolveDeviceTenant(uuid string) (string, error)
	}
}

func NewRepository(db *bun.DB, deviceRepo interface {
	ResolveDeviceTenant(uuid string) (string, error)
}) *Repository {
	return &Repository{db: db, deviceRepo: deviceRepo}
}

func NewWithDevice(db *bun.DB, deviceRepo *device.Repository) *Repository {
	return NewRepository(db, deviceRepo)
}

func (r *Repository) AppendMetric(uuid string, point inter.MetricPoint) error {
	tenantID, err := r.deviceRepo.ResolveDeviceTenant(uuid)
	if err != nil {
		tenantID = bunrepo.DefaultTenantID
	}

	_, err = r.db.NewInsert().
		Model(&bunrepo.MetricRow{
			UUID:     uuid,
			TenantID: tenantID,
			TS:       point.Timestamp,
			Value:    point.Value,
			Type:     point.Type,
		}).
		Returning("NULL").
		Exec(context.Background())
	return err
}

func (r *Repository) BatchAppendMetrics(uuid string, points []inter.MetricPoint) error {
	if len(points) == 0 {
		return nil
	}

	tenantID, err := r.deviceRepo.ResolveDeviceTenant(uuid)
	if err != nil {
		tenantID = bunrepo.DefaultTenantID
	}

	rows := make([]bunrepo.MetricRow, 0, len(points))
	for _, point := range points {
		rows = append(rows, bunrepo.MetricRow{
			UUID:     uuid,
			TenantID: tenantID,
			TS:       point.Timestamp,
			Value:    point.Value,
			Type:     point.Type,
		})
	}

	return r.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewInsert().
			Model(&rows).
			Returning("NULL").
			Exec(ctx)
		return err
	})
}

func (r *Repository) QueryMetrics(uuid string, start, end int64) ([]inter.MetricPoint, error) {
	var rows []bunrepo.MetricRow
	err := r.db.NewSelect().
		Model(&rows).
		Column("ts", "value", "type").
		Where("uuid = ?", uuid).
		Where("ts BETWEEN ? AND ?", start, end).
		OrderExpr("ts ASC").
		Scan(context.Background())
	if err != nil {
		return nil, err
	}
	return bunrepo.ToMetricPoints(rows), nil
}

func (r *Repository) QueryMetricsByTenant(tenantID, uuid string, start, end int64) ([]inter.MetricPoint, error) {
	var rows []bunrepo.MetricRow
	err := r.db.NewSelect().
		Model(&rows).
		Column("ts", "value", "type").
		Where("tenant_id = ?", bunrepo.NormalizeTenantID(tenantID)).
		Where("uuid = ?", uuid).
		Where("ts BETWEEN ? AND ?", start, end).
		OrderExpr("ts ASC").
		Scan(context.Background())
	if err != nil {
		return nil, err
	}
	return bunrepo.ToMetricPoints(rows), nil
}

func (r *Repository) WriteLog(uuid string, level string, message string) error {
	tenantID, err := r.deviceRepo.ResolveDeviceTenant(uuid)
	if err != nil {
		tenantID = bunrepo.DefaultTenantID
	}

	_, err = r.db.NewInsert().
		Model(&bunrepo.LogRow{
			UUID:     uuid,
			TenantID: tenantID,
			Level:    level,
			Message:  message,
		}).
		Returning("NULL").
		Exec(context.Background())
	return err
}
