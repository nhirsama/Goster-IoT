package device_manager

import (
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// DownlinkCommandService 负责下行命令的落库、入队和状态推进。
type DownlinkCommandService struct {
	dataStore inter.DataStore
	queue     inter.DeviceCommandQueue
}

// NewDownlinkCommandService 创建默认的下行命令编排服务。
func NewDownlinkCommandService(ds inter.DataStore, queue inter.DeviceCommandQueue) inter.DownlinkCommandService {
	return &DownlinkCommandService{
		dataStore: ds,
		queue:     queue,
	}
}

// Enqueue 创建命令记录并把下行消息推入设备队列。
func (s *DownlinkCommandService) Enqueue(scope inter.Scope, uuid string, cmdID inter.CmdID, command string, payloadJSON []byte) (inter.DownlinkMessage, error) {
	commandID, err := s.dataStore.CreateDeviceCommandByTenant(scope.TenantID, uuid, cmdID, command, payloadJSON)
	if err != nil {
		return inter.DownlinkMessage{}, err
	}

	msg := inter.DownlinkMessage{
		CommandID: commandID,
		CmdID:     cmdID,
		Payload:   payloadJSON,
	}
	if err := s.queue.Enqueue(uuid, msg); err != nil {
		_ = s.MarkFailed(commandID, err.Error())
		return inter.DownlinkMessage{}, err
	}
	return msg, nil
}

// PopDownlink 从设备队列中获取待发送命令。
func (s *DownlinkCommandService) PopDownlink(uuid string) (inter.DownlinkMessage, bool, error) {
	return s.queue.Dequeue(uuid)
}

// MarkSent 标记下行命令已发往设备。
func (s *DownlinkCommandService) MarkSent(commandID int64) error {
	if commandID <= 0 {
		return nil
	}
	return s.dataStore.UpdateDeviceCommandStatus(commandID, inter.DeviceCommandStatusSent, "")
}

// MarkAcked 标记下行命令已被设备确认。
func (s *DownlinkCommandService) MarkAcked(commandID int64) error {
	if commandID <= 0 {
		return nil
	}
	return s.dataStore.UpdateDeviceCommandStatus(commandID, inter.DeviceCommandStatusAcked, "")
}

// MarkFailed 标记下行命令发送失败。
func (s *DownlinkCommandService) MarkFailed(commandID int64, errorText string) error {
	if commandID <= 0 {
		return nil
	}
	return s.dataStore.UpdateDeviceCommandStatus(commandID, inter.DeviceCommandStatusFailed, strings.TrimSpace(errorText))
}
