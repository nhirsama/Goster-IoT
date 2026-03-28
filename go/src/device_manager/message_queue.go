package device_manager

import (
	"errors"
	"sync"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// InMemoryDeviceCommandQueue 是设备下行队列的默认内存实现。
type InMemoryDeviceCommandQueue struct {
	mu       sync.Mutex
	queues   map[string][]inter.DownlinkMessage
	capacity int
}

// NewDeviceCommandQueue 创建默认内存态的设备下行队列。
func NewDeviceCommandQueue(cap int) inter.DeviceCommandQueue {
	return &InMemoryDeviceCommandQueue{
		queues:   make(map[string][]inter.DownlinkMessage),
		capacity: cap,
	}
}

func (m *InMemoryDeviceCommandQueue) Enqueue(uuid string, message inter.DownlinkMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue := append([]inter.DownlinkMessage(nil), m.queues[uuid]...)
	if m.capacity > 0 && len(queue) >= m.capacity {
		// 队列满策略：丢弃最早的一条并压入新指令。
		queue = append(queue[1:], message)
		m.queues[uuid] = queue
		return nil
	}
	m.queues[uuid] = append(queue, message)
	return nil
}

func (m *InMemoryDeviceCommandQueue) Requeue(uuid string, message inter.DownlinkMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue := append([]inter.DownlinkMessage(nil), m.queues[uuid]...)
	if m.capacity > 0 && len(queue) >= m.capacity {
		// 重试命令优先级更高，队列满时淘汰最新的一条待发消息，保住重试项。
		queue = queue[:len(queue)-1]
	}
	if m.capacity > 0 && len(queue) >= m.capacity {
		return errors.New("队列已满且无法回退")
	}
	queue = append([]inter.DownlinkMessage{message}, queue...)
	m.queues[uuid] = queue
	return nil
}

func (m *InMemoryDeviceCommandQueue) Dequeue(uuid string) (inter.DownlinkMessage, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue := m.queues[uuid]
	if len(queue) == 0 {
		return inter.DownlinkMessage{}, false, nil
	}
	msg := queue[0]
	queue = queue[1:]
	if len(queue) == 0 {
		delete(m.queues, uuid)
	} else {
		m.queues[uuid] = queue
	}
	return msg, true, nil
}

func (m *InMemoryDeviceCommandQueue) IsEmpty(uuid string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.queues[uuid]) == 0
}
