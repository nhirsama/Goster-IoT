package runtime_test

import (
	"context"
	"testing"
	"time"

	"github.com/aarondl/authboss/v3"
	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/persistence"
	storageruntime "github.com/nhirsama/Goster-IoT/src/storage/runtime"
)

func openSQLiteRuntimeStore(t *testing.T) *storageruntime.Store {
	t.Helper()

	dbPath := t.TempDir() + "/runtime.db"
	if err := persistence.EnsureSchema(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   dbPath,
	}); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	store, err := storageruntime.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite runtime store failed: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func TestRuntimeStoreDeviceAndTelemetryFlow(t *testing.T) {
	store := openSQLiteRuntimeStore(t)

	meta := inter.DeviceMetadata{
		Name:               "runtime-device",
		Token:              "runtime-token",
		AuthenticateStatus: inter.Authenticated,
	}
	if err := store.InitDevice("runtime-device-1", meta); err != nil {
		t.Fatalf("InitDevice failed: %v", err)
	}

	tenantID, err := store.ResolveDeviceTenant("runtime-device-1")
	if err != nil {
		t.Fatalf("ResolveDeviceTenant failed: %v", err)
	}
	if tenantID != "tenant_legacy" {
		t.Fatalf("unexpected tenant id: got=%s want=%s", tenantID, "tenant_legacy")
	}

	if err := store.AppendMetric("runtime-device-1", inter.MetricPoint{Timestamp: 1000, Value: 12.5, Type: 1}); err != nil {
		t.Fatalf("AppendMetric failed: %v", err)
	}
	if err := store.BatchAppendMetrics("runtime-device-1", []inter.MetricPoint{
		{Timestamp: 2000, Value: 40.1, Type: 2},
		{Timestamp: 3000, Value: 88.3, Type: 4},
	}); err != nil {
		t.Fatalf("BatchAppendMetrics failed: %v", err)
	}
	if err := store.WriteLog("runtime-device-1", "warn", "test-log"); err != nil {
		t.Fatalf("WriteLog failed: %v", err)
	}

	points, err := store.QueryMetricsByTenant("tenant_legacy", "runtime-device-1", 900, 3100)
	if err != nil {
		t.Fatalf("QueryMetricsByTenant failed: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("unexpected metric count: %+v", points)
	}
}

func TestRuntimeStoreUserAndTenantRoleFlow(t *testing.T) {
	dbPath := t.TempDir() + "/user.db"
	if err := persistence.EnsureSchema(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   dbPath,
	}); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}
	userAuthStore, err := persistence.OpenAuthStore(appcfg.DBConfig{
		Driver:     "sqlite",
		Path:       dbPath,
		SchemaMode: "managed",
	})
	if err != nil {
		t.Fatalf("OpenAuthStore failed: %v", err)
	}
	t.Cleanup(func() {
		_ = persistence.CloseIfPossible(userAuthStore)
	})

	storer, ok := userAuthStore.(authboss.CreatingServerStorer)
	if !ok {
		t.Fatal("auth store does not implement CreatingServerStorer")
	}
	if err := storer.Create(context.Background(), &datastore.AuthUser{
		Username: "member",
		Password: "plain_pw_for_tests",
	}); err != nil && err != authboss.ErrUserFound {
		t.Fatalf("Create user failed: %v", err)
	}

	userStore, err := storageruntime.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite runtime store failed: %v", err)
	}
	t.Cleanup(func() {
		_ = userStore.Close()
	})

	if err := userStore.UpdateUserPermission("member", inter.PermissionReadWrite); err != nil {
		t.Fatalf("UpdateUserPermission failed: %v", err)
	}
	roles, err := userStore.GetUserTenantRoles("member")
	if err != nil {
		t.Fatalf("GetUserTenantRoles failed: %v", err)
	}
	if roles["tenant_legacy"] != inter.TenantRoleRW {
		t.Fatalf("unexpected tenant role map: %+v", roles)
	}
}

func TestRuntimeStoreDeviceCommandFlow(t *testing.T) {
	store := openSQLiteRuntimeStore(t)

	if err := store.InitDevice("runtime-device-2", inter.DeviceMetadata{
		Name:               "cmd-device",
		Token:              "cmd-token",
		AuthenticateStatus: inter.Authenticated,
	}); err != nil {
		t.Fatalf("InitDevice failed: %v", err)
	}

	commandID, err := store.CreateDeviceCommand("runtime-device-2", inter.CmdActionExec, "action_exec", []byte(`{"on":true}`))
	if err != nil {
		t.Fatalf("CreateDeviceCommand failed: %v", err)
	}
	if commandID <= 0 {
		t.Fatalf("unexpected command id: %d", commandID)
	}
	if err := store.UpdateDeviceCommandStatus(commandID, inter.DeviceCommandStatusAcked, ""); err != nil {
		t.Fatalf("UpdateDeviceCommandStatus failed: %v", err)
	}
}

func TestRuntimeStoreExternalEntityFlow(t *testing.T) {
	store := openSQLiteRuntimeStore(t)

	entity := inter.ExternalEntity{
		Source:      "ha",
		EntityID:    "sensor.temp_1",
		Domain:      "sensor",
		Name:        "Temp 1",
		ValueType:   "number",
		LastStateTS: time.Now().UnixMilli(),
	}
	if err := store.UpsertExternalEntity(entity); err != nil {
		t.Fatalf("UpsertExternalEntity failed: %v", err)
	}

	got, err := store.GetExternalEntity("ha", "sensor.temp_1")
	if err != nil {
		t.Fatalf("GetExternalEntity failed: %v", err)
	}
	if got.Name != "Temp 1" {
		t.Fatalf("unexpected entity: %+v", got)
	}

	if err := store.BatchAppendExternalObservations([]inter.ExternalObservation{{
		Source:    "ha",
		EntityID:  "sensor.temp_1",
		Timestamp: time.Now().UnixMilli(),
		ValueNum:  floatPtr(23.5),
		Unit:      "C",
	}}); err != nil {
		t.Fatalf("BatchAppendExternalObservations failed: %v", err)
	}

	items, err := store.QueryExternalObservations("ha", "sensor.temp_1", 1, time.Now().UnixMilli()+1000, 10)
	if err != nil {
		t.Fatalf("QueryExternalObservations failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one external observation")
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
