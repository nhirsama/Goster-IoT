package DataStore

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"stm32f103keshe/src/inter"
)

// 辅助函数：创建临时存储目录
func setupTestStore(t *testing.T) (*LocalStore, string) {
	tempDir, err := os.MkdirTemp("", "datastore_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	store, err := NewLocalStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	return store, tempDir
}

// --- 功能测试 (Unit Tests) ---

func TestDeviceLifecycle(t *testing.T) {
	store, tempDir := setupTestStore(t)
	defer os.RemoveAll(tempDir)

	uuid := "dev-001"
	meta := inter.DeviceMetadata{
		Name:         "TestNode",
		SerialNumber: "SN123456",
	}

	// 1. 测试初始化
	t.Run("InitDevice", func(t *testing.T) {
		err := store.InitDevice(uuid, meta)
		if err != nil {
			t.Errorf("InitDevice failed: %v", err)
		}
		// 验证目录是否存在
		if _, err := os.Stat(filepath.Join(tempDir, uuid)); os.IsNotExist(err) {
			t.Error("Device directory was not created")
		}
	})

	// 2. 测试元数据读取
	t.Run("GetMetadata", func(t *testing.T) {
		readMeta, err := store.GetMetadata(uuid)
		if err != nil || readMeta.Name != meta.Name {
			t.Errorf("Metadata mismatch. Got %v, want %v", readMeta, meta)
		}
	})

	// 3. 测试时序数据存取
	t.Run("AppendAndQueryMetrics", func(t *testing.T) {
		now := time.Now().Unix()
		f := rand.Float64()
		points := []inter.MetricPoint{
			{Timestamp: now, Value: float32(f)},
			{Timestamp: now + 1, Value: 26.0},
		}

		for _, p := range points {
			store.AppendMetric(uuid, p.Timestamp, p.Value)
		}

		results, err := store.QueryMetrics(uuid, now, now+1)
		if err != nil || len(results) != 2 {
			t.Errorf("Query failed. Got %d points, err: %v", len(results), err)
		}
		if results[0].Value != float32(f) {
			t.Errorf("Value mismatch. Got %f", results[0].Value)
		}
	})

	// 4. 测试清理
	t.Run("DestroyDevice", func(t *testing.T) {
		store.DestroyDevice(uuid)
		if _, err := os.Stat(filepath.Join(tempDir, uuid)); !os.IsNotExist(err) {
			t.Error("Device directory still exists after destruction")
		}
	})
}

// --- 压力与并发测试 (Stress Tests) ---

func TestConcurrentAppend(t *testing.T) {
	store, tempDir := setupTestStore(t)
	defer os.RemoveAll(tempDir)

	uuid := "stress-uuid"
	store.InitDevice(uuid, inter.DeviceMetadata{Name: "StressNode"})

	const (
		goroutines = 50  // 50个并发协程
		pointsPerG = 200 // 每个协程写200条
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	startSignal := make(chan struct{})

	// 并发写入
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			<-startSignal // 同步起跑
			for j := 0; j < pointsPerG; j++ {
				ts := int64(id*10000 + j)
				store.AppendMetric(uuid, ts, float32(j))
			}
		}(i)
	}

	close(startSignal)
	wg.Wait()

	// 验证总数：50 * 200 = 10000 条数据，每条 12 字节 = 120,000 字节
	fInfo, _ := os.Stat(filepath.Join(tempDir, uuid, "metrics.bin"))
	expectedSize := int64(goroutines * pointsPerG * 12)
	if fInfo.Size() != expectedSize {
		t.Errorf("File size mismatch. Got %d, want %d", fInfo.Size(), expectedSize)
	}
}

// --- 基准测试 (Benchmarks) ---

func BenchmarkAppendMetric(b *testing.B) {
	store, tempDir := setupTestStore(nil)
	defer os.RemoveAll(tempDir)
	uuid := "bench-uuid"
	store.InitDevice(uuid, inter.DeviceMetadata{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.AppendMetric(uuid, int64(i), 1.23)
	}
}
