package datastore

import (
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestDeviceLifecycle(t *testing.T) {
	store := newTestStore(t)
	uuid := generateUUID("device-lifecycle")
	meta := generateRandomMeta()

	if err := store.InitDevice(uuid, meta); err != nil {
		t.Fatalf("InitDevice failed: %v", err)
	}

	loaded, err := store.LoadConfig(uuid)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if loaded.SerialNumber != meta.SerialNumber || loaded.Token != meta.Token {
		t.Fatalf("LoadConfig mismatch: got serial=%s token=%s", loaded.SerialNumber, loaded.Token)
	}

	meta.Name = "Updated-" + randomString(5)
	meta.AuthenticateStatus = inter.Authenticated
	if err := store.SaveMetadata(uuid, meta); err != nil {
		t.Fatalf("SaveMetadata failed: %v", err)
	}

	loaded, err = store.LoadConfig(uuid)
	if err != nil {
		t.Fatalf("LoadConfig after SaveMetadata failed: %v", err)
	}
	if loaded.Name != meta.Name || loaded.AuthenticateStatus != inter.Authenticated {
		t.Fatalf("SaveMetadata mismatch: got name=%s status=%v", loaded.Name, loaded.AuthenticateStatus)
	}

	if err := store.DestroyDevice(uuid); err != nil {
		t.Fatalf("DestroyDevice failed: %v", err)
	}
	if _, err := store.LoadConfig(uuid); err == nil {
		t.Fatal("expected LoadConfig to fail after DestroyDevice")
	}
}

func TestMetricsManagement(t *testing.T) {
	store := newTestStore(t)
	uuid := generateUUID("device-metrics")
	if err := store.InitDevice(uuid, generateRandomMeta()); err != nil {
		t.Fatalf("InitDevice failed: %v", err)
	}

	base := time.Now().UnixMilli()
	for i := 0; i < 100; i++ {
		p := inter.MetricPoint{
			Timestamp: base + int64(i),
			Value:     float32(i),
			Type:      0,
		}
		if err := store.AppendMetric(uuid, p); err != nil {
			t.Fatalf("AppendMetric failed at %d: %v", i, err)
		}
	}

	points, err := store.QueryMetrics(uuid, base+20, base+50)
	if err != nil {
		t.Fatalf("QueryMetrics failed: %v", err)
	}
	if len(points) != 31 {
		t.Fatalf("QueryMetrics length mismatch: got=%d want=31", len(points))
	}
	for _, p := range points {
		if p.Timestamp < base+20 || p.Timestamp > base+50 {
			t.Fatalf("point timestamp out of range: %d", p.Timestamp)
		}
	}
}

func TestAuthentication(t *testing.T) {
	store := newTestStore(t)
	uuid := generateUUID("device-auth")
	meta := generateRandomMeta()

	if err := store.InitDevice(uuid, meta); err != nil {
		t.Fatalf("InitDevice failed: %v", err)
	}

	gotUUID, status, err := store.GetDeviceByToken(meta.Token)
	if err != nil {
		t.Fatalf("GetDeviceByToken failed: %v", err)
	}
	if gotUUID != uuid || status != inter.AuthenticatePending {
		t.Fatalf("GetDeviceByToken mismatch: got uuid=%s status=%v", gotUUID, status)
	}

	newToken := "tk-new-" + randomString(16)
	if err := store.UpdateToken(uuid, newToken); err != nil {
		t.Fatalf("UpdateToken failed: %v", err)
	}
	if _, _, err := store.GetDeviceByToken(meta.Token); err == nil {
		t.Fatal("expected old token to be invalid")
	}

	gotUUID, _, err = store.GetDeviceByToken(newToken)
	if err != nil || gotUUID != uuid {
		t.Fatalf("new token authentication failed, uuid=%s err=%v", gotUUID, err)
	}
}

func TestListDevicesAndStatusFilter(t *testing.T) {
	store := newTestStore(t)
	total := 6
	for i := 0; i < total; i++ {
		meta := generateRandomMeta()
		if i%2 == 0 {
			meta.AuthenticateStatus = inter.Authenticated
		}
		if err := store.InitDevice(generateUUID("list"), meta); err != nil {
			t.Fatalf("InitDevice failed: %v", err)
		}
	}

	page1, err := store.ListDevices(1, 4)
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}
	if len(page1) == 0 || len(page1) > 4 {
		t.Fatalf("ListDevices returned unexpected size: %d", len(page1))
	}

	filtered, err := store.ListDevicesByStatus(inter.Authenticated, 1, 10)
	if err != nil {
		t.Fatalf("ListDevicesByStatus failed: %v", err)
	}
	if len(filtered) != total/2 {
		t.Fatalf("ListDevicesByStatus count mismatch: got=%d want=%d", len(filtered), total/2)
	}
}

func TestWriteLog(t *testing.T) {
	store := newTestStore(t)
	uuid := generateUUID("device-log")
	if err := store.InitDevice(uuid, generateRandomMeta()); err != nil {
		t.Fatalf("InitDevice failed: %v", err)
	}

	for _, level := range []string{"info", "warn", "error"} {
		if err := store.WriteLog(uuid, level, "message-"+randomString(8)); err != nil {
			t.Fatalf("WriteLog failed for %s: %v", level, err)
		}
	}
}
