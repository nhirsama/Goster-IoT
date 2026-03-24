package device_manager

import (
	"errors"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// DevicePresenceService 负责设备心跳与在线状态判定。
type DevicePresenceService struct {
	store    inter.DevicePresenceStore
	deadline time.Duration
}

// NewDevicePresenceWithStore 创建设备在线状态服务。
func NewDevicePresenceWithStore(deadline time.Duration, store inter.DevicePresenceStore) *DevicePresenceService {
	if store == nil {
		store = NewInMemoryDevicePresenceStore()
	}
	if deadline <= 0 {
		deadline = 60 * time.Second
	}
	return &DevicePresenceService{
		store:    store,
		deadline: deadline,
	}
}

// SetDeadline 允许装配层或测试动态调整在线判定阈值。
func (s *DevicePresenceService) SetDeadline(deadline time.Duration) {
	if deadline > 0 {
		s.deadline = deadline
	}
}

// HandleHeartbeat 记录最新心跳时间。
func (s *DevicePresenceService) HandleHeartbeat(uuid string) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return
	}
	s.store.SaveLastSeen(uuid, time.Now())
}

// QueryDeviceStatus 根据最近一次心跳时间计算设备逻辑在线状态。
func (s *DevicePresenceService) QueryDeviceStatus(uuid string) (inter.DeviceStatus, error) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return inter.StatusOffline, errors.New("设备标识为空")
	}

	lastSeen, ok := s.store.LoadLastSeen(uuid)
	if !ok {
		return inter.StatusOffline, errors.New("设备从未上线")
	}

	delta := time.Since(lastSeen)
	if delta < s.deadline {
		return inter.StatusOnline, nil
	}
	if delta < 2*s.deadline {
		return inter.StatusDelayed, nil
	}
	return inter.StatusOffline, nil
}

func (s *DevicePresenceService) delete(uuid string) {
	s.store.Delete(uuid)
}
