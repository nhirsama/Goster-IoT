package device_manager

import (
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestInMemoryDeviceCommandQueueMaintainsFIFO(t *testing.T) {
	queue := NewDeviceCommandQueue(2)

	first := inter.DownlinkMessage{CommandID: 1, CmdID: inter.CmdActionExec}
	second := inter.DownlinkMessage{CommandID: 2, CmdID: inter.CmdActionExec}

	if err := queue.Enqueue("device-1", first); err != nil {
		t.Fatalf("enqueue first failed: %v", err)
	}
	if err := queue.Enqueue("device-1", second); err != nil {
		t.Fatalf("enqueue second failed: %v", err)
	}

	got, ok, err := queue.Dequeue("device-1")
	if err != nil {
		t.Fatalf("dequeue first failed: %v", err)
	}
	if !ok || got.CommandID != first.CommandID {
		t.Fatalf("unexpected first message: %+v ok=%v", got, ok)
	}

	got, ok, err = queue.Dequeue("device-1")
	if err != nil {
		t.Fatalf("dequeue second failed: %v", err)
	}
	if !ok || got.CommandID != second.CommandID {
		t.Fatalf("unexpected second message: %+v ok=%v", got, ok)
	}

	if !queue.IsEmpty("device-1") {
		t.Fatal("queue should be empty after dequeue")
	}
}

func TestInMemoryDeviceCommandQueueDropsOldestWhenFull(t *testing.T) {
	queue := NewDeviceCommandQueue(2)

	for _, id := range []int64{1, 2, 3} {
		if err := queue.Enqueue("device-1", inter.DownlinkMessage{CommandID: id, CmdID: inter.CmdActionExec}); err != nil {
			t.Fatalf("enqueue %d failed: %v", id, err)
		}
	}

	got, ok, err := queue.Dequeue("device-1")
	if err != nil {
		t.Fatalf("dequeue first failed: %v", err)
	}
	if !ok || got.CommandID != 2 {
		t.Fatalf("expected oldest message to be dropped, got %+v ok=%v", got, ok)
	}

	got, ok, err = queue.Dequeue("device-1")
	if err != nil {
		t.Fatalf("dequeue second failed: %v", err)
	}
	if !ok || got.CommandID != 3 {
		t.Fatalf("expected latest message to remain, got %+v ok=%v", got, ok)
	}
}

func TestInMemoryDeviceCommandQueueRequeueToFront(t *testing.T) {
	queue := NewDeviceCommandQueue(3)

	first := inter.DownlinkMessage{CommandID: 1, CmdID: inter.CmdActionExec}
	second := inter.DownlinkMessage{CommandID: 2, CmdID: inter.CmdActionExec}
	retry := inter.DownlinkMessage{CommandID: 99, CmdID: inter.CmdActionExec}

	if err := queue.Enqueue("device-1", first); err != nil {
		t.Fatalf("enqueue first failed: %v", err)
	}
	if err := queue.Enqueue("device-1", second); err != nil {
		t.Fatalf("enqueue second failed: %v", err)
	}
	if err := queue.Requeue("device-1", retry); err != nil {
		t.Fatalf("requeue failed: %v", err)
	}

	got, ok, err := queue.Dequeue("device-1")
	if err != nil {
		t.Fatalf("dequeue retry failed: %v", err)
	}
	if !ok || got.CommandID != retry.CommandID {
		t.Fatalf("expected retried message first, got %+v ok=%v", got, ok)
	}

	got, ok, err = queue.Dequeue("device-1")
	if err != nil {
		t.Fatalf("dequeue first failed: %v", err)
	}
	if !ok || got.CommandID != first.CommandID {
		t.Fatalf("expected original first message second, got %+v ok=%v", got, ok)
	}
}
