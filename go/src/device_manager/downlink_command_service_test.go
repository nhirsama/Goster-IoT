package device_manager

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/persistence"
)

func TestDownlinkCommandServiceEnqueueAndStatusFlow(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "downlink.db")
	ds, err := persistence.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to init runtime store: %v", err)
	}
	t.Cleanup(func() {
		_ = persistence.CloseIfPossible(ds)
	})

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

func (f failingQueue) Requeue(uuid string, message inter.DownlinkMessage) error {
	return errors.New("queue failed")
}

func (f failingQueue) Dequeue(uuid string) (inter.DownlinkMessage, bool, error) {
	return inter.DownlinkMessage{}, false, nil
}

func (f failingQueue) IsEmpty(uuid string) bool { return true }

func TestDownlinkCommandServiceMarkFailedAndQueueError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "downlink_failed.db")
	ds, err := persistence.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to init runtime store: %v", err)
	}
	t.Cleanup(func() {
		_ = persistence.CloseIfPossible(ds)
	})

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

func TestDownlinkCommandServiceRequeueRestoresQueuedStatus(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "downlink_requeue.db")
	ds, err := persistence.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to init runtime store: %v", err)
	}
	t.Cleanup(func() {
		_ = persistence.CloseIfPossible(ds)
	})

	uuid := "device-3"
	if err := ds.InitDevice(uuid, inter.DeviceMetadata{
		Name:               "Device 3",
		SerialNumber:       "sn-3",
		MACAddress:         "mac-3",
		Token:              "tk-3",
		AuthenticateStatus: inter.Authenticated,
	}); err != nil {
		t.Fatalf("failed to init device: %v", err)
	}

	queue := NewDeviceCommandQueue(8)
	service := NewDownlinkCommandService(ds, queue)
	msg, err := service.Enqueue(inter.Scope{}, uuid, inter.CmdActionExec, "action_exec", []byte(`{"op":"retry"}`))
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	popped, ok, err := service.PopDownlink(uuid)
	if err != nil {
		t.Fatalf("pop failed: %v", err)
	}
	if !ok || popped.CommandID != msg.CommandID {
		t.Fatalf("unexpected popped message: %+v ok=%v", popped, ok)
	}

	if err := service.MarkSent(msg.CommandID); err != nil {
		t.Fatalf("mark sent failed: %v", err)
	}
	if err := service.Requeue(uuid, popped); err != nil {
		t.Fatalf("requeue failed: %v", err)
	}

	retry, ok, err := service.PopDownlink(uuid)
	if err != nil {
		t.Fatalf("pop retry failed: %v", err)
	}
	if !ok || retry.CommandID != msg.CommandID {
		t.Fatalf("unexpected retried message: %+v ok=%v", retry, ok)
	}

	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	var status string
	if err := db.QueryRow("SELECT status FROM integration_external_commands WHERE id = ?", msg.CommandID).Scan(&status); err != nil {
		t.Fatalf("failed to query command status: %v", err)
	}
	if status != string(inter.DeviceCommandStatusQueued) {
		t.Fatalf("unexpected command status after requeue: got %s want %s", status, inter.DeviceCommandStatusQueued)
	}
}
