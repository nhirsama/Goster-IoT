package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/storage/command"
	"github.com/nhirsama/Goster-IoT/src/storage/device"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/testhelper"
)

func TestRepositoryCreateAndUpdateDeviceCommand(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "command_repo.db")
	deviceRepo := device.NewRepository(base.DB)
	repo := command.NewRepository(base.DB, deviceRepo)

	if err := deviceRepo.InitDevice("command-device", inter.DeviceMetadata{
		Name:               "command-device",
		Token:              "command-token",
		AuthenticateStatus: inter.Authenticated,
	}); err != nil {
		t.Fatalf("InitDevice failed: %v", err)
	}

	commandID, err := repo.CreateDeviceCommand("command-device", inter.CmdActionExec, "action_exec", []byte(`{"on":true}`))
	if err != nil {
		t.Fatalf("CreateDeviceCommand failed: %v", err)
	}
	if commandID <= 0 {
		t.Fatalf("unexpected command id: %d", commandID)
	}

	if err := repo.UpdateDeviceCommandStatus(commandID, inter.DeviceCommandStatusAcked, ""); err != nil {
		t.Fatalf("UpdateDeviceCommandStatus failed: %v", err)
	}

	var row struct {
		Status string `bun:"status"`
		Source string `bun:"source"`
	}
	if err := base.DB.NewRaw(
		"SELECT status, source FROM integration_external_commands WHERE id = ?",
		commandID,
	).Scan(context.Background(), &row); err != nil {
		t.Fatalf("query command row failed: %v", err)
	}
	if row.Status != string(inter.DeviceCommandStatusAcked) || row.Source != "goster_device" {
		t.Fatalf("unexpected command row: %+v", row)
	}
}

func TestRepositoryCreateDeviceCommandByTenantRejectsMismatch(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "command_repo_mismatch.db")
	deviceRepo := device.NewRepository(base.DB)
	repo := command.NewRepository(base.DB, deviceRepo)

	if err := deviceRepo.InitDevice("command-device-2", inter.DeviceMetadata{
		Name:               "command-device-2",
		Token:              "command-token-2",
		AuthenticateStatus: inter.Authenticated,
	}); err != nil {
		t.Fatalf("InitDevice failed: %v", err)
	}

	_, err := repo.CreateDeviceCommandByTenant("tenant_other", "command-device-2", inter.CmdActionExec, "action_exec", []byte(`{}`))
	if err == nil {
		t.Fatal("expected tenant mismatch error")
	}
	if !errors.Is(err, inter.ErrDeviceTenantMismatch) {
		t.Fatalf("expected tenant mismatch sentinel, got %v", err)
	}
}
