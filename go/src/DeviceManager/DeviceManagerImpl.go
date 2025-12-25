package DeviceManager

import (
	"errors"
	"sync"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

type DeviceManager struct {
	DataStore     inter.DataStore
	DeviceManager inter.DeviceManager
	timer         sync.Map
	DeathLine     time.Duration
}

func NewDeviceManager(ds inter.DataStore, deviceManager inter.DeviceManager) inter.DeviceManager {
	return &DeviceManager{
		DataStore:     ds,
		DeviceManager: deviceManager,
		timer:         sync.Map{},
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

type MessageQueue struct {
	queues   sync.Map
	capacity int
}

func NewMessageQueue(cap int) (inter.MessageQueue, error) {
	return &MessageQueue{
		capacity: cap,
	}, nil
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
