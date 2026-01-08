package device_manager

import (
	"errors"
	"sync"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

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
		// 队列满策略：丢弃最早的一条并压入新指令
		select {
		case <-q: // 弹出最早的
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
		return true
	}
	q := actual.(chan interface{})
	return len(q) == 0
}
