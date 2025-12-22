package DataStore

import (
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// --- 随机数据生成辅助函数 ---

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))]
	}
	return string(b)
}

func setupTestStore(t *testing.T) (*LocalStore, string) {
	tempDir, err := os.MkdirTemp("", "datastore_token_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	store, err := NewLocalStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	return store, tempDir
}

// --- 核心功能测试 ---

func TestTokenAndDeviceMapping(t *testing.T) {
	store, tempDir := setupTestStore(t)
	defer os.RemoveAll(tempDir)

	// 生成随机测试数据
	randUUID := "dev-" + randomString(8)
	randToken := "tk-" + randomString(16)
	randName := "Node-" + randomString(4)

	meta := inter.DeviceMetadata{
		Name:  randName,
		Token: randToken,
	}

	// 1. 测试初始化与自动索引
	t.Run("InitAndAutoIndex", func(t *testing.T) {
		err := store.InitDevice(randUUID, meta)
		if err != nil {
			t.Fatalf("InitDevice failed: %v", err)
		}

		// 验证通过 Token 是否能找回 UUID
		foundUUID, err := store.GetDeviceByToken(randToken)
		if err != nil {
			t.Errorf("GetDeviceByToken failed: %v", err)
		}
		if foundUUID != randUUID {
			t.Errorf("Mapping mismatch: got %s, want %s", foundUUID, randUUID)
		}
	})

	// 2. 测试 Token 更新逻辑
	t.Run("UpdateTokenMapping", func(t *testing.T) {
		newToken := "new-tk-" + randomString(16)
		err := store.UpdateToken(randUUID, newToken)
		if err != nil {
			t.Fatalf("UpdateToken failed: %v", err)
		}

		// 验证旧 Token 应该失效
		_, err = store.GetDeviceByToken(randToken)
		if err == nil {
			t.Error("Old token should be invalidated but still works")
		}

		// 验证新 Token 应该生效
		foundUUID, err := store.GetDeviceByToken(newToken)
		if err != nil || foundUUID != randUUID {
			t.Errorf("New token mapping failed: %v", err)
		}

		// 验证磁盘上的 config.json 也同步更新了
		updatedMeta, _ := store.GetMetadata(randUUID)
		if updatedMeta.Token != newToken {
			t.Errorf("Metadata Token not updated in config.json: got %s", updatedMeta.Token)
		}
	})
}

// --- 并发压力测试 ---

func TestConcurrentTokenAccess(t *testing.T) {
	store, tempDir := setupTestStore(t)
	defer os.RemoveAll(tempDir)

	const deviceCount = 20
	const opsPerDevice = 50
	var wg sync.WaitGroup

	// 1. 并发初始化多个设备
	for i := 0; i < deviceCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			u := fmt.Sprintf("u-%d-%s", id, randomString(4))
			tk := "t-" + randomString(12)
			store.InitDevice(u, inter.DeviceMetadata{Name: "Device", Token: tk})
		}(i)
	}
	wg.Wait()

	// 2. 模拟高并发下的 Token 更新与查询（测试全局 tokenIndex 锁）
	startSignal := make(chan struct{})
	for i := 0; i < deviceCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-startSignal
			// 随机尝试获取或更新 Token
			for j := 0; j < opsPerDevice; j++ {
				dummyToken := "test-" + randomString(10)
				_, _ = store.GetDeviceByToken(dummyToken) // 频繁读

				// 模拟业务中偶尔的 Token 刷新
				if j%10 == 0 {
					_ = store.UpdateToken(fmt.Sprintf("dev-%d", id), "new-"+randomString(10))
				}
			}
		}(i)
	}

	close(startSignal)
	wg.Wait()
	t.Log("Concurrent Token access completed without race/panic")
}

// --- 时序数据随机化测试 ---

func TestAppendMetricsRandomized(t *testing.T) {
	store, tempDir := setupTestStore(t)
	defer os.RemoveAll(tempDir)

	uuid := "metric-dev-" + randomString(4)
	store.InitDevice(uuid, inter.DeviceMetadata{Name: "Sensor"})

	// 生成随机数量的数据点
	count := 50 + rand.IntN(50)
	startTime := time.Now().Unix()

	t.Logf("Appending %d random metric points", count)

	for i := 0; i < count; i++ {
		ts := startTime + int64(i)
		val := rand.Float32() * 100.0
		err := store.AppendMetric(uuid, ts, val)
		if err != nil {
			t.Fatalf("Append failed at index %d: %v", i, err)
		}
	}

	// 查询并校验
	res, err := store.QueryMetrics(uuid, startTime, startTime+int64(count))
	if err != nil || len(res) != count {
		t.Errorf("Query mismatch: got %d points, want %d", len(res), count)
	}
}

// --- 基准测试：Token 查找性能 ---

func BenchmarkGetDeviceByToken(b *testing.B) {
	store, tempDir := setupTestStore(nil)
	defer os.RemoveAll(tempDir)

	// 预填充 100 个设备
	tokens := make([]string, 1000)
	for i := 0; i < 100; i++ {
		tokens[i] = "tk-" + randomString(20)
		store.InitDevice(fmt.Sprintf("dev-%d", i), inter.DeviceMetadata{Token: tokens[i]})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 随机查找其中一个 Token
		target := tokens[rand.IntN(len(tokens))]
		_, _ = store.GetDeviceByToken(target)
	}
}
