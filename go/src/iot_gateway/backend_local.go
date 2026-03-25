package iot_gateway

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// localBackend 是 IoT Gateway 在单体部署下的本地后端适配器。
// 它把网络层请求转接到当前进程内的核心业务与存储实现。
type localBackend struct {
	registry         inter.DeviceRegistry
	presence         inter.DevicePresence
	telemetry        inter.TelemetryIngestService
	downlinkCommands inter.DownlinkCommandService
}

func newLocalBackend(registry inter.DeviceRegistry, presence inter.DevicePresence, telemetry inter.TelemetryIngestService, downlinkCommands inter.DownlinkCommandService) inter.GatewayBackend {
	return &localBackend{
		registry:         registry,
		presence:         presence,
		telemetry:        telemetry,
		downlinkCommands: downlinkCommands,
	}
}

func (b *localBackend) AuthenticateDevice(token string) (string, error) {
	return b.registry.Authenticate(token)
}

func (b *localBackend) RegisterDevice(meta inter.DeviceMetadata) (inter.DeviceRegistrationResult, error) {
	uuid := b.registry.GenerateUUID(meta)
	existingMeta, err := b.registry.GetDeviceMetadata(uuid)
	if err != nil {
		if registerErr := b.registry.RegisterDevice(meta); registerErr != nil {
			return inter.DeviceRegistrationResult{}, fmt.Errorf("init device failed: %w", registerErr)
		}
		return inter.DeviceRegistrationResult{
			Status: inter.RegistrationPending,
			UUID:   uuid,
		}, nil
	}

	switch existingMeta.AuthenticateStatus {
	case inter.AuthenticatePending:
		return inter.DeviceRegistrationResult{
			Status: inter.RegistrationPending,
			UUID:   uuid,
		}, nil
	case inter.AuthenticateRefuse:
		return inter.DeviceRegistrationResult{
			Status: inter.RegistrationRejected,
			UUID:   uuid,
		}, errors.New("device registration refused")
	case inter.Authenticated:
		return inter.DeviceRegistrationResult{
			Status: inter.RegistrationAccepted,
			UUID:   uuid,
			Token:  existingMeta.Token,
		}, nil
	default:
		return inter.DeviceRegistrationResult{
			Status: inter.RegistrationRejected,
			UUID:   uuid,
		}, errors.New("unknown auth status")
	}
}

func (b *localBackend) ReportHeartbeat(uuid string) error {
	if strings.TrimSpace(uuid) == "" {
		return errors.New("uuid is required")
	}
	b.presence.HandleHeartbeat(uuid)
	return nil
}

func (b *localBackend) ReportMetrics(uuid string, points []inter.MetricPoint) error {
	return b.telemetry.IngestMetrics(uuid, points)
}

func (b *localBackend) ReportLog(uuid string, data inter.LogUploadData) error {
	return b.telemetry.IngestLog(uuid, data)
}

func (b *localBackend) ReportEvent(uuid string, payload []byte) error {
	return b.telemetry.IngestEvent(uuid, payload)
}

func (b *localBackend) ReportDeviceError(uuid string, payload []byte) error {
	return b.telemetry.IngestDeviceError(uuid, payload)
}

func (b *localBackend) PopDownlink(uuid string) (inter.DownlinkMessage, bool, error) {
	return b.downlinkCommands.PopDownlink(uuid)
}

func (b *localBackend) MarkDownlinkSent(commandID int64) error {
	return b.downlinkCommands.MarkSent(commandID)
}

func (b *localBackend) MarkDownlinkAcked(commandID int64) error {
	return b.downlinkCommands.MarkAcked(commandID)
}

func (b *localBackend) MarkDownlinkFailed(commandID int64, errorText string) error {
	return b.downlinkCommands.MarkFailed(commandID, errorText)
}
