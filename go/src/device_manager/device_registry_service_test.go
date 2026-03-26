package device_manager

import (
	"testing"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/persistence"
	storageruntime "github.com/nhirsama/Goster-IoT/src/storage/runtime"
)

func openRuntimeStoreForDeviceRegistry(t *testing.T) *storageruntime.Store {
	t.Helper()

	dbPath := t.TempDir() + "/device_registry.db"
	if err := persistence.EnsureSchema(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   dbPath,
	}); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}
	store, err := storageruntime.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func TestDeviceRegistryServiceLifecycle(t *testing.T) {
	store := openRuntimeStoreForDeviceRegistry(t)
	registry := NewDeviceRegistry(store)

	meta := inter.DeviceMetadata{
		Name:         "device-registry",
		SerialNumber: "sn-1",
		MACAddress:   "mac-1",
	}
	uuid := registry.GenerateUUID(meta)
	if uuid == "" {
		t.Fatal("GenerateUUID should not be empty")
	}

	if err := registry.RegisterDevice(meta); err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}

	loaded, err := registry.GetDeviceMetadata(uuid)
	if err != nil {
		t.Fatalf("GetDeviceMetadata failed: %v", err)
	}
	if loaded.AuthenticateStatus != inter.AuthenticatePending || loaded.Token != "" {
		t.Fatalf("unexpected registered metadata: %+v", loaded)
	}

	if _, err := registry.Authenticate("missing-token"); err != inter.ErrInvalidToken {
		t.Fatalf("expected invalid token, got %v", err)
	}

	if err := registry.ApproveDevice(uuid); err != nil {
		t.Fatalf("ApproveDevice failed: %v", err)
	}
	loaded, err = registry.GetDeviceMetadata(uuid)
	if err != nil {
		t.Fatalf("GetDeviceMetadata(after approve) failed: %v", err)
	}
	if loaded.Token == "" {
		t.Fatal("approved device should have token")
	}

	authUUID, err := registry.Authenticate(loaded.Token)
	if err != nil || authUUID != uuid {
		t.Fatalf("Authenticate failed: uuid=%s err=%v", authUUID, err)
	}

	newToken, err := registry.RefreshToken(uuid)
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}
	if newToken == "" || newToken == loaded.Token {
		t.Fatalf("unexpected refreshed token: old=%s new=%s", loaded.Token, newToken)
	}

	status := inter.Authenticated
	items, err := registry.ListDevices(&status, 1, 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("ListDevices failed: items=%+v err=%v", items, err)
	}

	items, err = registry.ListDevicesByScope(inter.Scope{TenantID: "tenant_legacy"}, &status, 1, 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("ListDevicesByScope failed: items=%+v err=%v", items, err)
	}

	scopedMeta, err := registry.GetDeviceMetadataByScope(inter.Scope{TenantID: "tenant_legacy"}, uuid)
	if err != nil || scopedMeta.Name != meta.Name {
		t.Fatalf("GetDeviceMetadataByScope failed: meta=%+v err=%v", scopedMeta, err)
	}

	if err := registry.RevokeToken(uuid); err != nil {
		t.Fatalf("RevokeToken failed: %v", err)
	}
	loaded, err = registry.GetDeviceMetadata(uuid)
	if err != nil {
		t.Fatalf("GetDeviceMetadata(after revoke) failed: %v", err)
	}
	if loaded.Token != "" || loaded.AuthenticateStatus != inter.AuthenticateRevoked {
		t.Fatalf("unexpected revoked metadata: %+v", loaded)
	}

	if err := registry.RejectDevice(uuid); err != nil {
		t.Fatalf("RejectDevice failed: %v", err)
	}
	if err := registry.UnblockDevice(uuid); err != nil {
		t.Fatalf("UnblockDevice failed: %v", err)
	}
}

func TestDeviceRegistryDeleteClearsPresenceState(t *testing.T) {
	store := openRuntimeStoreForDeviceRegistry(t)
	presence := NewDevicePresenceWithStore(time.Second, NewInMemoryDevicePresenceStore())

	deletedUUID := ""
	registry := NewDeviceRegistryWithHooks(store, DeviceRegistryHooks{
		OnDelete: func(uuid string) {
			deletedUUID = uuid
			presence.RemoveDevice(uuid)
		},
	})

	meta := inter.DeviceMetadata{
		Name:         "device-delete",
		SerialNumber: "sn-delete",
		MACAddress:   "mac-delete",
	}
	uuid := registry.GenerateUUID(meta)
	if err := registry.RegisterDevice(meta); err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}
	if err := registry.DeleteDevice(uuid); err != nil {
		t.Fatalf("DeleteDevice failed: %v", err)
	}
	if deletedUUID != uuid {
		t.Fatalf("delete hook not triggered: %s", deletedUUID)
	}

	meta2 := inter.DeviceMetadata{
		Name:         "presence-device",
		SerialNumber: "sn-presence",
		MACAddress:   "mac-presence",
	}
	uuid2 := registry.GenerateUUID(meta2)
	if err := registry.RegisterDevice(meta2); err != nil {
		t.Fatalf("registry RegisterDevice failed: %v", err)
	}
	if err := registry.ApproveDevice(uuid2); err != nil {
		t.Fatalf("registry ApproveDevice failed: %v", err)
	}
	presence.HandleHeartbeat(uuid2)
	if status, err := presence.QueryDeviceStatus(uuid2); err != nil || status != inter.StatusOnline {
		t.Fatalf("unexpected device status: status=%v err=%v", status, err)
	}

	presence.SetDeadline(time.Nanosecond)
	time.Sleep(2 * time.Millisecond)
	if status, err := presence.QueryDeviceStatus(uuid2); err != nil || status == inter.StatusOnline {
		t.Fatalf("expected non-online status after deadline: status=%v err=%v", status, err)
	}

	if err := registry.DeleteDevice(uuid2); err != nil {
		t.Fatalf("registry DeleteDevice failed: %v", err)
	}
	if status, err := presence.QueryDeviceStatus(uuid2); err == nil || status != inter.StatusOffline {
		t.Fatalf("presence should be cleared after delete: status=%v err=%v", status, err)
	}
}
