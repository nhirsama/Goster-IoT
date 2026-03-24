package device_manager

import (
	"strings"
	"sync"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// InMemoryDevicePresenceStore 是设备在线状态的默认内存实现。
type InMemoryDevicePresenceStore struct {
	lastSeen sync.Map
}

// NewInMemoryDevicePresenceStore 创建默认内存态在线状态存储。
func NewInMemoryDevicePresenceStore() inter.DevicePresenceStore {
	return &InMemoryDevicePresenceStore{}
}

func (s *InMemoryDevicePresenceStore) SaveLastSeen(uuid string, at time.Time) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return
	}
	s.lastSeen.Store(uuid, at)
}

func (s *InMemoryDevicePresenceStore) LoadLastSeen(uuid string) (time.Time, bool) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return time.Time{}, false
	}
	value, ok := s.lastSeen.Load(uuid)
	if !ok {
		return time.Time{}, false
	}
	seenAt, ok := value.(time.Time)
	if !ok {
		return time.Time{}, false
	}
	return seenAt, true
}

func (s *InMemoryDevicePresenceStore) Delete(uuid string) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return
	}
	s.lastSeen.Delete(uuid)
}
