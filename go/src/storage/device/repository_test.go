package device_test

import (
	"context"
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/storage/device"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/testhelper"
)

func TestRepositoryDeviceLifecycleAndScopedQueries(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "device_repo.db")
	repo := device.NewRepository(base.DB)

	meta := inter.DeviceMetadata{
		Name:               "device-1",
		Token:              "token-1",
		AuthenticateStatus: inter.AuthenticatePending,
	}
	if err := repo.InitDevice("uuid-1", meta); err != nil {
		t.Fatalf("InitDevice failed: %v", err)
	}

	loaded, err := repo.LoadConfig("uuid-1")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if loaded.Name != meta.Name || loaded.Token != meta.Token {
		t.Fatalf("unexpected metadata: %+v", loaded)
	}

	if err := repo.UpdateToken("uuid-1", "token-2"); err != nil {
		t.Fatalf("UpdateToken failed: %v", err)
	}
	uuid, status, err := repo.GetDeviceByToken("token-2")
	if err != nil {
		t.Fatalf("GetDeviceByToken failed: %v", err)
	}
	if uuid != "uuid-1" || status != inter.AuthenticatePending {
		t.Fatalf("unexpected token lookup: uuid=%s status=%v", uuid, status)
	}

	devices, err := repo.ListDevicesByTenant("tenant_legacy", nil, 1, 10)
	if err != nil {
		t.Fatalf("ListDevicesByTenant failed: %v", err)
	}
	if len(devices) != 1 || devices[0].UUID != "uuid-1" {
		t.Fatalf("unexpected scoped device list: %+v", devices)
	}
}

func TestRepositoryDestroyDeviceAlsoDeletesMetricsAndLogs(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "device_destroy.db")
	repo := device.NewRepository(base.DB)

	if err := repo.InitDevice("uuid-2", inter.DeviceMetadata{
		Name:               "device-2",
		Token:              "token-3",
		AuthenticateStatus: inter.Authenticated,
	}); err != nil {
		t.Fatalf("InitDevice failed: %v", err)
	}

	if _, err := base.DB.NewRaw(
		"INSERT INTO metrics (uuid, tenant_id, ts, value, type) VALUES (?, ?, ?, ?, ?)",
		"uuid-2", "tenant_legacy", int64(1000), 12.3, 1,
	).Exec(context.Background()); err != nil {
		t.Fatalf("seed metrics failed: %v", err)
	}
	if _, err := base.DB.NewRaw(
		"INSERT INTO logs (uuid, tenant_id, level, message) VALUES (?, ?, ?, ?)",
		"uuid-2", "tenant_legacy", "info", "seed-log",
	).Exec(context.Background()); err != nil {
		t.Fatalf("seed logs failed: %v", err)
	}

	if err := repo.DestroyDevice("uuid-2"); err != nil {
		t.Fatalf("DestroyDevice failed: %v", err)
	}

	var metricCount int
	if err := base.DB.NewRaw("SELECT COUNT(*) FROM metrics WHERE uuid = ?", "uuid-2").Scan(context.Background(), &metricCount); err != nil {
		t.Fatalf("count metrics failed: %v", err)
	}
	if metricCount != 0 {
		t.Fatalf("metrics should be deleted, got=%d", metricCount)
	}

	var logCount int
	if err := base.DB.NewRaw("SELECT COUNT(*) FROM logs WHERE uuid = ?", "uuid-2").Scan(context.Background(), &logCount); err != nil {
		t.Fatalf("count logs failed: %v", err)
	}
	if logCount != 0 {
		t.Fatalf("logs should be deleted, got=%d", logCount)
	}
}
