package DataStore

import (
	"path/filepath"
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
	// 确保引入 sqlite 驱动，如果主文件已经引了，这里其实可以省略，但为了保险
	_ "modernc.org/sqlite"
)

// setupTestStore 是一个辅助函数，用于创建临时的真实数据库环境
// 它利用 t.TempDir() 创建临时目录，测试结束后操作系统会自动清理
func setupTestStore(t *testing.T) (*AtomStore, string) {
	// 1. 获取临时数据库路径
	tempDir := t.TempDir()
	//tempDir := "./data"
	dbPath := filepath.Join(tempDir, "test_iot.db")

	// 2. 初始化 Store
	ds, err := NewAtomStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}

	store, ok := ds.(*AtomStore)
	if !ok {
		t.Fatalf("Returned interface is not *AtomStore")
	}

	return store, dbPath
}

func TestAtomStore_DeviceLifecycle(t *testing.T) {
	store, _ := setupTestStore(t)
	// 测试结束后关闭连接
	defer store.db.Close()

	targetUUID := "uuid-test-001"
	initMeta := inter.DeviceMetadata{
		Name:               "Test Device",
		HWVersion:          "v1.0",
		SWVersion:          "v1.0.1",
		ConfigVersion:      "c-100",
		SerialNumber:       "SN-001",
		MACAddress:         "AA:BB:CC:DD:EE:FF",
		Token:              "token-secret-123",
		AuthenticateStatus: 1, // 假设 1 代表已认证
	}

	t.Run("InitDevice", func(t *testing.T) {
		err := store.InitDevice(targetUUID, initMeta)
		if err != nil {
			t.Fatalf("InitDevice failed: %v", err)
		}
	})

	t.Run("LoadConfig", func(t *testing.T) {
		loaded, err := store.LoadConfig(targetUUID)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		if loaded.Name != initMeta.Name {
			t.Errorf("Expected name %s, got %s", initMeta.Name, loaded.Name)
		}
		if loaded.Token != initMeta.Token {
			t.Errorf("Expected token %s, got %s", initMeta.Token, loaded.Token)
		}
	})

	t.Run("SaveMetadata (Update)", func(t *testing.T) {
		newMeta := initMeta
		newMeta.Name = "Updated Device Name"
		newMeta.SWVersion = "v2.0"

		err := store.SaveMetadata(targetUUID, newMeta)
		if err != nil {
			t.Fatalf("SaveMetadata failed: %v", err)
		}

		// 验证更新是否生效
		loaded, err := store.LoadConfig(targetUUID)
		if err != nil {
			t.Fatalf("LoadConfig after update failed: %v", err)
		}
		if loaded.Name != "Updated Device Name" {
			t.Errorf("Update failed, name is %s", loaded.Name)
		}
	})
}

func TestAtomStore_Metrics(t *testing.T) {
	store, _ := setupTestStore(t)
	defer store.db.Close()

	uuid := "sensor-001"
	// 预先插入设备（虽然 metrics 表没有外键强约束，但符合逻辑）
	store.InitDevice(uuid, inter.DeviceMetadata{Name: "Sensor"})

	// 模拟插入数据：时间戳分别为 100, 200, 300
	metrics := []struct {
		ts  int64
		val float32
	}{
		{100, 10.5},
		{200, 20.5},
		{300, 30.5},
	}

	t.Run("AppendMetric", func(t *testing.T) {
		for _, m := range metrics {
			err := store.AppendMetric(uuid, m.ts, m.val)
			if err != nil {
				t.Errorf("Failed to append metric: %v", err)
			}
		}
	})

	t.Run("QueryMetrics", func(t *testing.T) {
		// 查询范围 150 - 350 (应该包含 200 和 300)
		points, err := store.QueryMetrics(uuid, 150, 350)
		if err != nil {
			t.Fatalf("QueryMetrics failed: %v", err)
		}

		if len(points) != 2 {
			t.Errorf("Expected 2 points, got %d", len(points))
		}

		if points[0].Timestamp != 200 || points[1].Timestamp != 300 {
			t.Errorf("Unexpected points data: %+v", points)
		}
	})
}

func TestAtomStore_TokenAuth(t *testing.T) {
	store, _ := setupTestStore(t)
	defer store.db.Close()

	uuid := "auth-device-001"
	token := "sk-live-token"
	authStatus := inter.AuthenticateStatusType(1)

	store.InitDevice(uuid, inter.DeviceMetadata{
		Token:              token,
		AuthenticateStatus: authStatus,
	})

	t.Run("GetDeviceByToken Success", func(t *testing.T) {
		gotUUID, gotStatus, err := store.GetDeviceByToken(token)
		if err != nil {
			t.Fatalf("GetDeviceByToken failed: %v", err)
		}
		if gotUUID != uuid {
			t.Errorf("UUID mismatch: expected %s, got %s", uuid, gotUUID)
		}
		if gotStatus != authStatus {
			t.Errorf("Status mismatch: expected %v, got %v", authStatus, gotStatus)
		}
	})

	t.Run("GetDeviceByToken Fail", func(t *testing.T) {
		_, _, err := store.GetDeviceByToken("invalid-token")
		if err == nil {
			t.Error("Expected error for invalid token, got nil")
		}
	})

	t.Run("UpdateToken", func(t *testing.T) {
		newToken := "sk-new-token"
		err := store.UpdateToken(uuid, newToken)
		if err != nil {
			t.Fatalf("UpdateToken failed: %v", err)
		}

		// 验证旧 Token 失效
		_, _, err = store.GetDeviceByToken(token)
		if err == nil {
			t.Error("Old token should not work")
		}

		// 验证新 Token 生效
		gotUUID, _, err := store.GetDeviceByToken(newToken)
		if err != nil || gotUUID != uuid {
			t.Errorf("New token failed to verify")
		}
	})
}

func TestAtomStore_DestroyDevice_Transaction(t *testing.T) {
	store, _ := setupTestStore(t)
	defer store.db.Close()

	uuid := "delete-me"

	// 1. 准备数据：设备信息、Metrics、Logs
	store.InitDevice(uuid, inter.DeviceMetadata{Name: "To Delete"})
	store.AppendMetric(uuid, 1000, 50.0)
	store.WriteLog(uuid, "INFO", "System start")

	// 2. 执行删除
	err := store.DestroyDevice(uuid)
	if err != nil {
		t.Fatalf("DestroyDevice failed: %v", err)
	}

	// 3. 验证是否全部删除 (Devices 表)
	_, err = store.LoadConfig(uuid)
	if err == nil {
		t.Error("Device should be deleted, but LoadConfig found it")
	}

	// 4. 验证 Metrics 表
	points, _ := store.QueryMetrics(uuid, 0, 2000)
	if len(points) > 0 {
		t.Error("Metrics should be deleted")
	}

	// 5. 验证 Logs 表 (手动查询)
	var logCount int
	store.db.QueryRow("SELECT COUNT(*) FROM logs WHERE uuid = ?", uuid).Scan(&logCount)
	if logCount > 0 {
		t.Error("Logs should be deleted")
	}
}

func TestAtomStore_ListDevices(t *testing.T) {
	store, _ := setupTestStore(t)
	defer store.db.Close()

	// 插入 15 条数据
	for i := 0; i < 15; i++ {
		// 为了测试分页，确保 token 唯一，uuid 唯一
		uuid := string(rune('A' + i)) // 简易生成 A, B, C...
		store.InitDevice(uuid, inter.DeviceMetadata{
			Name:  "Device " + uuid,
			Token: "token-" + uuid,
		})
	}

	// 测试分页：每页 10 条，取第 1 页
	page1, err := store.ListDevices(1, 10)
	if err != nil {
		t.Fatalf("ListDevices page 1 failed: %v", err)
	}
	if len(page1) != 10 {
		t.Errorf("Expected 10 devices on page 1, got %d", len(page1))
	}

	// 测试分页：每页 10 条，取第 2 页 (应该剩 5 条)
	page2, err := store.ListDevices(2, 10)
	if err != nil {
		t.Fatalf("ListDevices page 2 failed: %v", err)
	}
	if len(page2) != 5 {
		t.Errorf("Expected 5 devices on page 2, got %d", len(page2))
	}
}
