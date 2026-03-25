package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/bunrepo"
	"github.com/uptrace/bun"
)

type tenantResolver interface {
	ResolveDeviceTenant(uuid string) (string, error)
}

type Repository struct {
	db            *bun.DB
	deviceTenants tenantResolver
}

func NewRepository(db *bun.DB, deviceTenants tenantResolver) *Repository {
	return &Repository{db: db, deviceTenants: deviceTenants}
}

func (r *Repository) CreateDeviceCommand(uuid string, cmdID inter.CmdID, command string, payloadJSON []byte) (int64, error) {
	tenantID, err := r.deviceTenants.ResolveDeviceTenant(uuid)
	if err != nil {
		tenantID = bunrepo.DefaultTenantID
	}
	return r.createDeviceCommandRecord(tenantID, uuid, cmdID, command, payloadJSON, false)
}

func (r *Repository) CreateDeviceCommandByTenant(tenantID, uuid string, cmdID inter.CmdID, command string, payloadJSON []byte) (int64, error) {
	return r.createDeviceCommandRecord(tenantID, uuid, cmdID, command, payloadJSON, true)
}

func (r *Repository) createDeviceCommandRecord(tenantID, uuid string, cmdID inter.CmdID, command string, payloadJSON []byte, validateTenant bool) (int64, error) {
	tenantID = bunrepo.NormalizeTenantID(tenantID)
	uuid = strings.TrimSpace(uuid)
	command = strings.TrimSpace(strings.ToLower(command))
	if uuid == "" {
		return 0, errors.New("uuid is required")
	}
	if command == "" {
		return 0, errors.New("command is required")
	}
	if validateTenant {
		deviceTenant, err := r.deviceTenants.ResolveDeviceTenant(uuid)
		if err != nil {
			return 0, err
		}
		if bunrepo.NormalizeTenantID(deviceTenant) != tenantID {
			return 0, errors.New("device tenant mismatch")
		}
	}

	row := &bunrepo.DeviceCommandRow{
		TenantID:    tenantID,
		Source:      "goster_device",
		EntityID:    uuid,
		Command:     fmt.Sprintf("%s:%d", command, cmdID),
		PayloadJSON: bunrepo.PayloadStringPtr(payloadJSON),
		Status:      string(inter.DeviceCommandStatusQueued),
	}
	if _, err := r.db.NewInsert().
		Model(row).
		Returning("id").
		Exec(context.Background()); err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (r *Repository) UpdateDeviceCommandStatus(commandID int64, status inter.DeviceCommandStatus, errorText string) error {
	if commandID <= 0 {
		return errors.New("invalid command id")
	}
	if !isValidDeviceCommandStatus(status) {
		return errors.New("invalid command status")
	}

	var executedAt *time.Time
	if status == inter.DeviceCommandStatusAcked || status == inter.DeviceCommandStatusFailed {
		now := time.Now().UTC()
		executedAt = &now
	}
	errText := strings.TrimSpace(errorText)

	res, err := r.db.NewUpdate().
		Table("integration_external_commands").
		Set("status = ?", string(status)).
		Set("error_text = ?", bunrepo.NullableOptionalString(errText)).
		Set("executed_at = COALESCE(?, executed_at)", executedAt).
		Where("id = ?", commandID).
		Where("source = ?", "goster_device").
		Returning("NULL").
		Exec(context.Background())
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("device command not found")
	}
	return nil
}

func isValidDeviceCommandStatus(status inter.DeviceCommandStatus) bool {
	switch status {
	case inter.DeviceCommandStatusQueued, inter.DeviceCommandStatusSent, inter.DeviceCommandStatusAcked, inter.DeviceCommandStatusFailed:
		return true
	default:
		return false
	}
}
