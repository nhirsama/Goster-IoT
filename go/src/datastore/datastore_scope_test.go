package datastore

import (
	"strings"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestTenantScopedDeviceQueries(t *testing.T) {
	store := newTestStore(t)
	sqlStore := asSQLStore(t, store)

	metaA := generateRandomMeta()
	metaB := generateRandomMeta()
	uuidA := "tenant-a-" + strings.Repeat("a", 16)
	uuidB := "tenant-b-" + strings.Repeat("b", 16)

	if err := store.InitDevice(uuidA, metaA); err != nil {
		t.Fatalf("InitDevice A failed: %v", err)
	}
	if err := store.InitDevice(uuidB, metaB); err != nil {
		t.Fatalf("InitDevice B failed: %v", err)
	}

	if _, err := sqlStore.db.Exec("UPDATE devices SET tenant_id = ? WHERE uuid = ?", "tenant_a", uuidA); err != nil {
		t.Fatalf("set tenant for A failed: %v", err)
	}
	if _, err := sqlStore.db.Exec("UPDATE devices SET tenant_id = ? WHERE uuid = ?", "tenant_b", uuidB); err != nil {
		t.Fatalf("set tenant for B failed: %v", err)
	}

	tenantA, err := store.ResolveDeviceTenant(uuidA)
	if err != nil {
		t.Fatalf("ResolveDeviceTenant A failed: %v", err)
	}
	if tenantA != "tenant_a" {
		t.Fatalf("unexpected tenant for A: %s", tenantA)
	}

	itemsA, err := store.ListDevicesByTenant("tenant_a", nil, 1, 50)
	if err != nil {
		t.Fatalf("ListDevicesByTenant failed: %v", err)
	}
	if len(itemsA) != 1 || itemsA[0].UUID != uuidA {
		t.Fatalf("tenant_a device scope mismatch: %+v", itemsA)
	}

	if _, err := store.LoadConfigByTenant("tenant_b", uuidA); err == nil {
		t.Fatalf("cross-tenant LoadConfigByTenant should fail")
	}
}

func TestTenantScopedMetricsAndCommands(t *testing.T) {
	store := newTestStore(t)
	sqlStore := asSQLStore(t, store)

	uuid := "tenant-c-" + strings.Repeat("c", 16)
	meta := generateRandomMeta()
	if err := store.InitDevice(uuid, meta); err != nil {
		t.Fatalf("InitDevice failed: %v", err)
	}
	if _, err := sqlStore.db.Exec("UPDATE devices SET tenant_id = ? WHERE uuid = ?", "tenant_c", uuid); err != nil {
		t.Fatalf("set tenant failed: %v", err)
	}

	point := inter.MetricPoint{Timestamp: time.Now().UnixMilli(), Value: 22.3, Type: 1}
	if err := store.AppendMetric(uuid, point); err != nil {
		t.Fatalf("AppendMetric failed: %v", err)
	}

	points, err := store.QueryMetricsByTenant("tenant_c", uuid, point.Timestamp-10, point.Timestamp+10)
	if err != nil {
		t.Fatalf("QueryMetricsByTenant failed: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point in tenant scope, got=%d", len(points))
	}

	pointsWrongTenant, err := store.QueryMetricsByTenant("tenant_other", uuid, point.Timestamp-10, point.Timestamp+10)
	if err != nil {
		t.Fatalf("QueryMetricsByTenant wrong tenant should not error, got: %v", err)
	}
	if len(pointsWrongTenant) != 0 {
		t.Fatalf("wrong tenant should not see points, got=%d", len(pointsWrongTenant))
	}

	if _, err := store.CreateDeviceCommandByTenant("tenant_other", uuid, inter.CmdActionExec, "action_exec", []byte(`{"op":"reboot"}`)); err == nil {
		t.Fatalf("CreateDeviceCommandByTenant should reject cross-tenant request")
	}

	commandID, err := store.CreateDeviceCommandByTenant("tenant_c", uuid, inter.CmdActionExec, "action_exec", []byte(`{"op":"reboot"}`))
	if err != nil {
		t.Fatalf("CreateDeviceCommandByTenant failed: %v", err)
	}
	if commandID <= 0 {
		t.Fatalf("unexpected command id: %d", commandID)
	}

	var storedTenant string
	if err := sqlStore.db.QueryRow("SELECT tenant_id FROM integration_external_commands WHERE id = ?", commandID).Scan(&storedTenant); err != nil {
		t.Fatalf("query command tenant failed: %v", err)
	}
	if storedTenant != "tenant_c" {
		t.Fatalf("unexpected command tenant: %s", storedTenant)
	}
}
