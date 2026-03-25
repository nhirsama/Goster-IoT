package device_manager

import (
	"path/filepath"
	"testing"

	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestDownlinkCommandServiceEnqueueAndStatusFlow(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "downlink.db")
	ds, err := datastore.NewDataStoreSql(dbPath)
	if err != nil {
		t.Fatalf("failed to init datastore: %v", err)
	}

	queue := NewDeviceCommandQueue(8)
	service := NewDownlinkCommandService(ds, queue)

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

	msg, err := service.Enqueue(inter.Scope{}, uuid, inter.CmdActionExec, "action_exec", []byte(`{"op":"reboot"}`))
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if msg.CommandID <= 0 || msg.CmdID != inter.CmdActionExec {
		t.Fatalf("unexpected downlink message: %+v", msg)
	}

	popped, ok, err := service.PopDownlink(uuid)
	if err != nil {
		t.Fatalf("pop failed: %v", err)
	}
	if !ok {
		t.Fatal("expected queued downlink message")
	}
	if popped.CommandID != msg.CommandID {
		t.Fatalf("unexpected popped command id: got %d want %d", popped.CommandID, msg.CommandID)
	}

	if err := service.MarkSent(msg.CommandID); err != nil {
		t.Fatalf("mark sent failed: %v", err)
	}
	if err := service.MarkAcked(msg.CommandID); err != nil {
		t.Fatalf("mark acked failed: %v", err)
	}
}
