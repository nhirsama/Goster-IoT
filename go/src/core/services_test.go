package core

import (
	"path/filepath"
	"testing"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/persistence"
)

func TestNewServicesWithConfigBuildsAllCoreServices(t *testing.T) {
	ds, err := persistence.OpenSQLite(filepath.Join(t.TempDir(), "core_services.db"))
	if err != nil {
		t.Fatalf("failed to init datastore: %v", err)
	}

	services := NewServicesWithConfig(ds, appcfg.DeviceManagerConfig{
		QueueCapacity:     4,
		HeartbeatDeadline: 50 * time.Millisecond,
	})

	if services.DeviceRegistry == nil || services.DevicePresence == nil || services.ExternalEntities == nil {
		t.Fatal("core services should expose registry, presence and external entity services")
	}
	if services.TelemetryIngest == nil || services.DownlinkQueue == nil || services.DownlinkCommands == nil {
		t.Fatal("core services should expose telemetry and downlink services")
	}
}

func TestNewServicesDeleteDeviceClearsPresenceState(t *testing.T) {
	ds, err := persistence.OpenSQLite(filepath.Join(t.TempDir(), "core_presence.db"))
	if err != nil {
		t.Fatalf("failed to init datastore: %v", err)
	}

	services := NewServicesWithConfig(ds, appcfg.DeviceManagerConfig{
		HeartbeatDeadline: 100 * time.Millisecond,
	})

	uuid := "device-1"
	if err := ds.InitDevice(uuid, inter.DeviceMetadata{
		Name:               "Device 1",
		SerialNumber:       "sn-1",
		MACAddress:         "mac-1",
		Token:              "tk-1",
		AuthenticateStatus: inter.Authenticated,
	}); err != nil {
		t.Fatalf("failed to init device: %v", err)
	}

	services.DevicePresence.HandleHeartbeat(uuid)
	if _, err := services.DevicePresence.QueryDeviceStatus(uuid); err != nil {
		t.Fatalf("expected device to have presence state before delete: %v", err)
	}

	if err := services.DeviceRegistry.DeleteDevice(uuid); err != nil {
		t.Fatalf("delete device failed: %v", err)
	}

	if _, err := services.DevicePresence.QueryDeviceStatus(uuid); err == nil {
		t.Fatal("expected presence state to be cleared after delete")
	}
}
