package device_test

import (
	"context"
	"strings"
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

func TestRepositoryMetadataAndListQueries(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "device_metadata.db")
	repo := device.NewRepository(base.DB)

	if err := repo.InitDevice("uuid-a", inter.DeviceMetadata{
		Name:               "device-a",
		Token:              "token-a",
		AuthenticateStatus: inter.AuthenticatePending,
	}); err != nil {
		t.Fatalf("InitDevice uuid-a failed: %v", err)
	}
	if err := repo.InitDevice("uuid-b", inter.DeviceMetadata{
		Name:               "device-b",
		Token:              "token-b",
		AuthenticateStatus: inter.Authenticated,
	}); err != nil {
		t.Fatalf("InitDevice uuid-b failed: %v", err)
	}

	if _, err := base.DB.NewRaw(
		"UPDATE devices SET tenant_id = ?, created_at = ? WHERE uuid = ?",
		"tenant_other", "2026-01-01T00:00:00Z", "uuid-b",
	).Exec(context.Background()); err != nil {
		t.Fatalf("seed tenant for uuid-b failed: %v", err)
	}

	if err := repo.SaveMetadata("uuid-a", inter.DeviceMetadata{
		Name:               "device-a-updated",
		HWVersion:          "hw-1",
		SWVersion:          "sw-2",
		ConfigVersion:      "cfg-3",
		SerialNumber:       "sn-a",
		MACAddress:         "mac-a",
		Token:              "token-a-2",
		AuthenticateStatus: inter.Authenticated,
	}); err != nil {
		t.Fatalf("SaveMetadata failed: %v", err)
	}

	loaded, err := repo.LoadConfig("uuid-a")
	if err != nil {
		t.Fatalf("LoadConfig after update failed: %v", err)
	}
	if loaded.Name != "device-a-updated" || loaded.Token != "token-a-2" || loaded.AuthenticateStatus != inter.Authenticated {
		t.Fatalf("unexpected updated metadata: %+v", loaded)
	}

	allDevices, err := repo.ListDevices(0, 0)
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}
	if len(allDevices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(allDevices))
	}

	authenticatedOnly, err := repo.ListDevicesByStatus(inter.Authenticated, 1, 10)
	if err != nil {
		t.Fatalf("ListDevicesByStatus failed: %v", err)
	}
	if len(authenticatedOnly) != 2 {
		t.Fatalf("expected 2 authenticated devices after update, got %d", len(authenticatedOnly))
	}

	tenantID, err := repo.ResolveDeviceTenant("uuid-b")
	if err != nil {
		t.Fatalf("ResolveDeviceTenant failed: %v", err)
	}
	if tenantID != "tenant_other" {
		t.Fatalf("unexpected tenant id: got %s want tenant_other", tenantID)
	}

	scoped, err := repo.LoadConfigByTenant("tenant_other", "uuid-b")
	if err != nil {
		t.Fatalf("LoadConfigByTenant failed: %v", err)
	}
	if scoped.Name != "device-b" {
		t.Fatalf("unexpected scoped metadata: %+v", scoped)
	}
}

func TestRepositoryMissingDeviceBranches(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "device_missing.db")
	repo := device.NewRepository(base.DB)

	if err := repo.DestroyDevice("missing"); err == nil {
		t.Fatal("expected missing device destroy error")
	}

	if _, err := repo.LoadConfig("missing"); err == nil {
		t.Fatal("expected missing device load error")
	}

	if _, _, err := repo.GetDeviceByToken("missing-token"); err == nil {
		t.Fatal("expected missing token error")
	}

	if err := repo.UpdateToken("missing", "new-token"); err == nil {
		t.Fatal("expected missing device update token error")
	}

	if _, err := repo.ResolveDeviceTenant("missing"); err == nil {
		t.Fatal("expected missing tenant resolve error")
	}

	if _, err := repo.LoadConfigByTenant("tenant_legacy", "missing"); err == nil {
		t.Fatal("expected missing scoped load error")
	}
}

func TestRepositoryResolveDeviceTenantDefaultsLegacyWhenEmpty(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "device_tenant_default.db")
	repo := device.NewRepository(base.DB)

	if err := repo.InitDevice("uuid-c", inter.DeviceMetadata{
		Name:               "device-c",
		Token:              "token-c",
		AuthenticateStatus: inter.Authenticated,
	}); err != nil {
		t.Fatalf("InitDevice failed: %v", err)
	}

	if _, err := base.DB.NewRaw("UPDATE devices SET tenant_id = '' WHERE uuid = ?", "uuid-c").Exec(context.Background()); err != nil {
		t.Fatalf("clear tenant id failed: %v", err)
	}

	tenantID, err := repo.ResolveDeviceTenant("uuid-c")
	if err != nil {
		t.Fatalf("ResolveDeviceTenant failed: %v", err)
	}
	if tenantID != "tenant_legacy" {
		t.Fatalf("expected legacy fallback tenant, got %s", tenantID)
	}
}

func TestRepositoryGetDeviceByTokenMissingWrapsError(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "device_missing_wrap.db")
	repo := device.NewRepository(base.DB)

	_, _, err := repo.GetDeviceByToken("missing-token")
	if err == nil {
		t.Fatal("expected missing token error")
	}
	if !strings.Contains(err.Error(), "token not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
