package device_manager

import (
	"errors"
	"sync"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// InMemoryDeviceCommandQueue 是设备下行队列的默认内存实现。
type InMemoryDeviceCommandQueue struct {
	queues   sync.Map
	capacity int
}

// NewDeviceCommandQueue 创建默认内存态的设备下行队列。
func NewDeviceCommandQueue(cap int) inter.DeviceCommandQueue {
	return &InMemoryDeviceCommandQueue{
		capacity: cap,
	}
}

func (m *InMemoryDeviceCommandQueue) Enqueue(uuid string, message inter.DownlinkMessage) error {
	actual, _ := m.queues.LoadOrStore(uuid, make(chan inter.DownlinkMessage, m.capacity))
	q := actual.(chan inter.DownlinkMessage)

	select {
	case q <- message:
		return nil
	default:
		// 队列满策略：丢弃最早的一条并压入新指令。
		select {
		case <-q:
		default:
		}

		select {
		case q <- message:
			return nil
		default:
			return errors.New("队列已满且无法清理")
		}
	}
}

func (m *InMemoryDeviceCommandQueue) Dequeue(uuid string) (inter.DownlinkMessage, bool, error) {
	actual, exists := m.queues.Load(uuid)
	if !exists {
		return inter.DownlinkMessage{}, false, nil
	}
	q := actual.(chan inter.DownlinkMessage)
	select {
	case msg := <-q:
		return msg, true, nil
	default:
		return inter.DownlinkMessage{}, false, nil
	}
}

func (m *InMemoryDeviceCommandQueue) IsEmpty(uuid string) bool {
	actual, exists := m.queues.Load(uuid)
	if !exists {
		return true
	}
	q := actual.(chan inter.DownlinkMessage)
	return len(q) == 0
}
