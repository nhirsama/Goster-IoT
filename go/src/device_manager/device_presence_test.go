package device_manager

import (
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestDevicePresenceServiceTracksHeartbeatLifecycle(t *testing.T) {
	store := NewInMemoryDevicePresenceStore()
	service := NewDevicePresenceWithStore(50*time.Millisecond, store)

	if status, err := service.QueryDeviceStatus("dev-1"); err == nil || status != inter.StatusOffline {
		t.Fatalf("expected offline status with missing heartbeat, got status=%v err=%v", status, err)
	}

	service.HandleHeartbeat("dev-1")
	if status, err := service.QueryDeviceStatus("dev-1"); err != nil || status != inter.StatusOnline {
		t.Fatalf("expected online after heartbeat, got status=%v err=%v", status, err)
	}

	time.Sleep(70 * time.Millisecond)
	if status, err := service.QueryDeviceStatus("dev-1"); err != nil || status != inter.StatusDelayed {
		t.Fatalf("expected delayed after first deadline, got status=%v err=%v", status, err)
	}

	time.Sleep(50 * time.Millisecond)
	if status, err := service.QueryDeviceStatus("dev-1"); err != nil || status != inter.StatusOffline {
		t.Fatalf("expected offline after second deadline, got status=%v err=%v", status, err)
	}
}

func TestInMemoryDevicePresenceStoreDelete(t *testing.T) {
	store := NewInMemoryDevicePresenceStore()
	now := time.Now().UTC()

	store.SaveLastSeen("dev-1", now)
	if seenAt, ok := store.LoadLastSeen("dev-1"); !ok || !seenAt.Equal(now) {
		t.Fatalf("expected stored heartbeat, got seenAt=%v ok=%v", seenAt, ok)
	}

	store.Delete("dev-1")
	if _, ok := store.LoadLastSeen("dev-1"); ok {
		t.Fatal("expected heartbeat to be removed after delete")
	}
}
