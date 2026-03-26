package device_manager

import (
	"errors"
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

type failingQueue struct{}

func (f failingQueue) Enqueue(uuid string, message inter.DownlinkMessage) error {
	return errors.New("queue failed")
}

func (f failingQueue) Dequeue(uuid string) (inter.DownlinkMessage, bool, error) {
	return inter.DownlinkMessage{}, false, nil
}

func (f failingQueue) IsEmpty(uuid string) bool { return true }

func TestDownlinkCommandServiceMarkFailedAndQueueError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "downlink_failed.db")
	ds, err := datastore.NewDataStoreSql(dbPath)
	if err != nil {
		t.Fatalf("failed to init datastore: %v", err)
	}

	uuid := "device-2"
	if err := ds.InitDevice(uuid, inter.DeviceMetadata{
		Name:               "Device 2",
		SerialNumber:       "sn-2",
		MACAddress:         "mac-2",
		Token:              "tk-2",
		AuthenticateStatus: inter.Authenticated,
	}); err != nil {
		t.Fatalf("failed to init device: %v", err)
	}

	service := NewDownlinkCommandService(ds, failingQueue{})
	if _, err := service.Enqueue(inter.Scope{}, uuid, inter.CmdActionExec, "action_exec", []byte(`{"op":"reboot"}`)); err == nil {
		t.Fatal("expected enqueue to fail")
	}

	if err := service.MarkFailed(0, "ignored"); err != nil {
		t.Fatalf("mark failed with zero id should be ignored: %v", err)
	}
}
