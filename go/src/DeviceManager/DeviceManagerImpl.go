package DeviceManager

import (
	"errors"
	"sync"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

type DeviceManager struct {
	DataStore       inter.DataStore
	IdentityManager inter.IdentityManager
	timer           sync.Map
	message         inter.MessageQueue
	DeathLine       time.Duration
}

func NewDeviceManager(ds inter.DataStore, IdentityManager inter.IdentityManager) inter.DeviceManager {
	return &DeviceManager{
		DataStore:       ds,
		IdentityManager: IdentityManager,
		timer:           sync.Map{},
		message:         NewMessageQueue(100),
	}
}

func (d *DeviceManager) HandleHeartbeat(uuid string) {
	d.timer.Store(uuid, time.Now())
}

func (d *DeviceManager) QueryDeviceStatus(uuid string) (inter.DeviceStatus, error) {
	if val, ok := d.timer.Load(uuid); ok {
		if time.Now().Sub(val.(time.Time)) < d.DeathLine {
			return inter.StatusOnline, nil
		}
		return inter.StatusOffline, nil
	}
	return inter.StatusOffline, errors.New("设备未找到")
}

func (d *DeviceManager) QueuePush(uuid string, message interface{}) error {
	return d.message.Push(uuid, message)
}

func (d *DeviceManager) QueuePop(uuid string) (interface{}, bool) {
	return d.message.Pop(uuid)
}

func (d *DeviceManager) QueueIsEmpty(uuid string) bool {
	return d.message.IsEmpty(uuid)
}

type MessageQueue struct {
	queues   sync.Map
	capacity int
}

func NewMessageQueue(cap int) inter.MessageQueue {
	return &MessageQueue{
		capacity: cap,
	}
}

func (m *MessageQueue) Push(uuid string, message interface{}) error {
	actual, _ := m.queues.LoadOrStore(uuid, make(chan interface{}, m.capacity))
	q := actual.(chan interface{})

	select {
	case q <- message:
		return nil
	default:
		for {
			select {
			case <-q:
			default:
			}
			select {
			case q <- message:
				return nil
			default:
			}
		}
	}
}

func (m *MessageQueue) Pop(uuid string) (interface{}, bool) {
	actual, exists := m.queues.Load(uuid)
	if !exists {
		return nil, false
	}
	q := actual.(chan interface{})
	select {
	case msg := <-q:
		return msg, true
	default:
		return nil, false
	}
}

func (m *MessageQueue) IsEmpty(uuid string) bool {
	actual, exists := m.queues.Load(uuid)
	if !exists {
		return false
	}
	return len(actual.(chan interface{})) == 0
}
