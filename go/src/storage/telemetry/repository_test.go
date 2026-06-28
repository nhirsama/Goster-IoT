package telemetry_test

import (
	"context"
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/storage/device"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/testhelper"
	"github.com/nhirsama/Goster-IoT/src/storage/telemetry"
)

func TestRepositoryMetricsAndLogsUseResolvedTenant(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "telemetry_repo.db")
	deviceRepo := device.NewRepository(base.DB)
	repo := telemetry.NewWithDevice(base.DB, deviceRepo)

	if err := deviceRepo.InitDevice("telemetry-device", inter.DeviceMetadata{
		Name:               "telemetry-device",
		Token:              "telemetry-token",
		AuthenticateStatus: inter.Authenticated,
	}); err != nil {
		t.Fatalf("InitDevice failed: %v", err)
	}

	if err := repo.AppendMetric("telemetry-device", inter.MetricPoint{Timestamp: 1000, Value: 12.5, Type: 1}); err != nil {
		t.Fatalf("AppendMetric failed: %v", err)
	}
	if err := repo.BatchAppendMetrics("telemetry-device", []inter.MetricPoint{
		{Timestamp: 2000, Value: 40.1, Type: 2},
		{Timestamp: 3000, Value: 88.3, Type: 4},
	}); err != nil {
		t.Fatalf("BatchAppendMetrics failed: %v", err)
	}
	if err := repo.WriteLog("telemetry-device", "warn", "warn-msg"); err != nil {
		t.Fatalf("WriteLog failed: %v", err)
	}

	points, err := repo.QueryMetricsByTenant("tenant_legacy", "telemetry-device", 900, 3100)
	if err != nil {
		t.Fatalf("QueryMetricsByTenant failed: %v", err)
	}
	if len(points) != 3 || points[0].Timestamp != 1000 || points[2].Type != 4 {
		t.Fatalf("unexpected telemetry points: %+v", points)
	}

	var tenantID string
	if err := base.DB.NewRaw(
		"SELECT tenant_id FROM logs WHERE uuid = ? ORDER BY id DESC LIMIT 1",
		"telemetry-device",
	).Scan(context.Background(), &tenantID); err != nil {
		t.Fatalf("query log tenant failed: %v", err)
	}
	if tenantID != "tenant_legacy" {
		t.Fatalf("unexpected log tenant: %s", tenantID)
	}
}
