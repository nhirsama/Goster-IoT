package iot_gateway

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// localBackend 是 IoT Gateway 在单体部署下的本地后端适配器。
// 它把网络层请求转接到当前进程内的核心业务与存储实现。
type localBackend struct {
	dataStore     inter.DataStore
	deviceManager inter.DeviceManager
}

func newLocalBackend(ds inter.DataStore, dm inter.DeviceManager) inter.GatewayBackend {
	return &localBackend{
		dataStore:     ds,
		deviceManager: dm,
	}
}

func (b *localBackend) AuthenticateDevice(token string) (string, error) {
	return b.deviceManager.Authenticate(token)
}

func (b *localBackend) RegisterDevice(meta inter.DeviceMetadata) (inter.DeviceRegistrationResult, error) {
	uuid := b.deviceManager.GenerateUUID(meta)
	existingMeta, err := b.dataStore.LoadConfig(uuid)
	if err != nil {
		if registerErr := b.deviceManager.RegisterDevice(meta); registerErr != nil {
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
	b.deviceManager.HandleHeartbeat(uuid)
	return nil
}

func (b *localBackend) ReportMetrics(uuid string, points []inter.MetricPoint) error {
	return b.dataStore.BatchAppendMetrics(uuid, points)
}

func (b *localBackend) ReportLog(uuid string, data inter.LogUploadData) error {
	finalMsg := fmt.Sprintf("[%s] %s", time.UnixMilli(data.Timestamp).Format(time.DateTime), data.Message)
	return b.dataStore.WriteLog(uuid, mapLogLevel(data.Level), finalMsg)
}

func (b *localBackend) ReportEvent(uuid string, payload []byte) error {
	return b.dataStore.WriteLog(uuid, "EVENT", string(payload))
}

func (b *localBackend) ReportDeviceError(uuid string, payload []byte) error {
	return b.dataStore.WriteLog(uuid, "ERROR", string(payload))
}

func (b *localBackend) PopDownlink(uuid string) (inter.DownlinkMessage, bool, error) {
	msg, ok := b.deviceManager.QueuePop(uuid)
	if !ok {
		return inter.DownlinkMessage{}, false, nil
	}

	downlink, ok := msg.(inter.DownlinkMessage)
	if !ok {
		return inter.DownlinkMessage{}, false, errors.New("invalid downlink message type")
	}
	return downlink, true, nil
}

func (b *localBackend) MarkDownlinkSent(commandID int64) error {
	if commandID <= 0 {
		return nil
	}
	return b.dataStore.UpdateDeviceCommandStatus(commandID, inter.DeviceCommandStatusSent, "")
}

func (b *localBackend) MarkDownlinkAcked(commandID int64) error {
	if commandID <= 0 {
		return nil
	}
	return b.dataStore.UpdateDeviceCommandStatus(commandID, inter.DeviceCommandStatusAcked, "")
}

func (b *localBackend) MarkDownlinkFailed(commandID int64, errorText string) error {
	if commandID <= 0 {
		return nil
	}
	return b.dataStore.UpdateDeviceCommandStatus(commandID, inter.DeviceCommandStatusFailed, errorText)
}

func mapLogLevel(level inter.LogLevel) string {
	switch level {
	case inter.LogLevelDebug:
		return "DEBUG"
	case inter.LogLevelInfo:
		return "INFO"
	case inter.LogLevelWarn:
		return "WARN"
	case inter.LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
