package DataStore

import (
	"fmt"
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter" // 替换为你的 inter 包路径
)

// 全局存储接口实例
var store inter.DataStore

// TestMain 用于全局初始化（可选），这里我们在每个测试用例中懒加载
func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
	var err error
	store, err = NewDataStoreSql("./data.db")
	if err != nil {
		log.Fatal(err)
	}

	if store == nil {
		fmt.Println("警告: store 未初始化，请修改测试文件中的 TestMain 或 initStore")
		// 为了防止编译错误，这里如果不改代码会 panic，
		// 实际使用请取消注释下面的代码并填入你的构造函数
		// s, _ := sqlite.New("./data.db")
		// store = s
	}

	m.Run()
}

// -----------------------------------------------------------------------------
// 辅助函数：随机数据生成器
// -----------------------------------------------------------------------------

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func generateUUID() string {
	// 纳秒时间戳 + 随机数确保唯一性
	return fmt.Sprintf("test-device-%d-%d", time.Now().UnixNano(), rand.Intn(100000))
}

func generateRandomMeta() inter.DeviceMetadata {
	return inter.DeviceMetadata{
		Name:               "Device-" + randomString(6),
		HWVersion:          "v" + randomString(3),
		SWVersion:          "v" + randomString(3),
		ConfigVersion:      randomString(8),
		SerialNumber:       "SN" + randomString(12),
		MACAddress:         fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", rand.Intn(255), rand.Intn(255), rand.Intn(255), rand.Intn(255), rand.Intn(255), rand.Intn(255)),
		CreatedAt:          time.Now().Truncate(time.Second), // Truncate 避免数据库存储精度问题
		Token:              "tk-" + randomString(32),
		AuthenticateStatus: inter.AuthenticatePending,
	}
}

// -----------------------------------------------------------------------------
// 功能测试 (Functional Tests)
// -----------------------------------------------------------------------------

// TestDeviceLifecycle 测试完整的设备生命周期：创建 -> 读取 -> 更新 -> 删除
func TestDeviceLifecycle(t *testing.T) {
	if store == nil {
		t.Skip("Store not initialized")
	}

	uuid := generateUUID()
	meta := generateRandomMeta()

	// 1. InitDevice
	t.Run("InitDevice", func(t *testing.T) {
		err := store.InitDevice(uuid, meta)
		if err != nil {
			t.Fatalf("InitDevice failed: %v", err)
		}
	})

	// 2. LoadConfig (Verify Init)
	t.Run("LoadConfig", func(t *testing.T) {
		loaded, err := store.LoadConfig(uuid)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}
		// 简单的字段对比
		if loaded.SerialNumber != meta.SerialNumber || loaded.Token != meta.Token {
			t.Errorf("Loaded config mismatch. Got %+v, want %+v", loaded, meta)
		}
	})

	// 3. SaveMetadata (Update)
	newMeta := meta
	newMeta.Name = "Updated-" + randomString(5)
	newMeta.AuthenticateStatus = inter.Authenticated

	t.Run("SaveMetadata", func(t *testing.T) {
		err := store.SaveMetadata(uuid, newMeta)
		if err != nil {
			t.Fatalf("SaveMetadata failed: %v", err)
		}

		// Verify Update
		loaded, _ := store.LoadConfig(uuid)
		if loaded.Name != newMeta.Name {
			t.Errorf("Update failed. Name is %s, expected %s", loaded.Name, newMeta.Name)
		}
	})

	// 4. DestroyDevice
	t.Run("DestroyDevice", func(t *testing.T) {
		err := store.DestroyDevice(uuid)
		if err != nil {
			t.Fatalf("DestroyDevice failed: %v", err)
		}

		// Verify Destroy
		_, err = store.LoadConfig(uuid)
		if err == nil {
			t.Error("Device should be deleted, but LoadConfig returned success")
		}
	})
}

// TestMetricsManagement 测试时序数据的写入与查询
func TestMetricsManagement(t *testing.T) {
	if store == nil {
		t.Skip("Store not initialized")
	}

	uuid := generateUUID()
	store.InitDevice(uuid, generateRandomMeta())

	baseTime := time.Now().Unix()
	count := 100

	// 1. AppendMetric
	t.Run("AppendMetric", func(t *testing.T) {
		for i := 0; i < count; i++ {
			// 模拟每秒一个点，值随机
			ts := baseTime + int64(i)
			val := rand.Float32() * 100
			err := store.AppendMetric(uuid, inter.MetricPoint{Timestamp: ts, Value: val, Type: 0})
			if err != nil {
				t.Fatalf("AppendMetric failed at index %d: %v", i, err)
			}
		}
	})

	// 2. QueryMetrics
	t.Run("QueryMetrics", func(t *testing.T) {
		// 查询中间的一段数据 [20, 50]
		start := baseTime + 20
		end := baseTime + 50
		expectedCount := 31 // 闭区间 inclusive

		points, err := store.QueryMetrics(uuid, start, end)
		if err != nil {
			t.Fatalf("QueryMetrics failed: %v", err)
		}

		if len(points) != expectedCount {
			t.Errorf("QueryMetrics count mismatch. Got %d, want %d", len(points), expectedCount)
		}

		// 验证时间戳是否在范围内
		for _, p := range points {
			if p.Timestamp < start || p.Timestamp > end {
				t.Errorf("Point timestamp %d out of range [%d, %d]", p.Timestamp, start, end)
			}
		}
	})
}

// TestAuthentication 测试 Token 相关的逻辑
func TestAuthentication(t *testing.T) {
	if store == nil {
		t.Skip("Store not initialized")
	}

	uuid := generateUUID()
	meta := generateRandomMeta()
	originalToken := meta.Token

	store.InitDevice(uuid, meta)

	// 1. GetDeviceByToken
	t.Run("GetDeviceByToken", func(t *testing.T) {
		gotUUID, status, err := store.GetDeviceByToken(originalToken)
		if err != nil {
			t.Fatalf("GetDeviceByToken failed: %v", err)
		}
		if gotUUID != uuid {
			t.Errorf("UUID mismatch. Got %s, want %s", gotUUID, uuid)
		}
		if status != inter.AuthenticatePending {
			t.Errorf("Status mismatch. Got %v", status)
		}
	})

	// 2. UpdateToken
	newToken := "tk-new-" + randomString(20)
	t.Run("UpdateToken", func(t *testing.T) {
		err := store.UpdateToken(uuid, newToken)
		if err != nil {
			t.Fatalf("UpdateToken failed: %v", err)
		}

		// 验证旧 Token 失效
		_, _, err = store.GetDeviceByToken(originalToken)
		if err == nil {
			t.Error("Old token should be invalid now")
		}

		// 验证新 Token 生效
		gotUUID, _, err := store.GetDeviceByToken(newToken)
		if err != nil || gotUUID != uuid {
			t.Error("New token failed to authenticate")
		}

		// 验证 LoadConfig 也能读到新 Token
		loaded, _ := store.LoadConfig(uuid)
		if loaded.Token != newToken {
			t.Errorf("Metadata not updated with new token. Got %s", loaded.Token)
		}
	})
}

// TestPagination 测试设备列表分页
func TestPagination(t *testing.T) {
	if store == nil {
		t.Skip("Store not initialized")
	}

	// 创建一批特定的测试设备，使用特殊前缀以便区分
	prefix := "page-test-" + randomString(5)
	total := 15

	for i := 0; i < total; i++ {
		uuid := fmt.Sprintf("%s-%d", prefix, i)
		meta := generateRandomMeta()
		meta.Name = uuid
		store.InitDevice(uuid, meta)
	}

	// 测试分页
	pageSize := 5
	// 我们无法保证数据库里只有我们刚插入的这15个，因为测试是追加的
	// 所以我们主要测试返回的数量是否不超过 pageSize

	t.Run("ListDevices", func(t *testing.T) {
		list, err := store.ListDevices(1, pageSize)
		if err != nil {
			t.Fatalf("ListDevices failed: %v", err)
		}

		if len(list) > pageSize {
			t.Errorf("Return size %d exceeds page size %d", len(list), pageSize)
		}

		// 如果是全新的库，第一页应该满
		if len(list) == 0 {
			t.Error("Returned empty list")
		}
	})
}

// TestLogging 测试日志写入 (由于接口没有 ReadLog，只测写入不报错)
func TestLogging(t *testing.T) {
	if store == nil {
		t.Skip("Store not initialized")
	}

	uuid := generateUUID()
	store.InitDevice(uuid, generateRandomMeta())

	levels := []string{"info", "warn", "error"}
	for _, lvl := range levels {
		err := store.WriteLog(uuid, lvl, "Test log message "+randomString(10))
		if err != nil {
			t.Errorf("WriteLog failed for level %s: %v", lvl, err)
		}
	}
}

// TestUserManagement 测试用户注册、登录、修改密码
func TestUserManagement(t *testing.T) {
	if store == nil {
		t.Skip("Store not initialized")
	}

	username := "user_" + randomString(8)
	password := "pass_" + randomString(8)
	permission := inter.PermissionReadWrite

	// 1. RegisterUser
	t.Run("RegisterUser", func(t *testing.T) {
		err := store.RegisterUser(username, password, permission)
		if err != nil {
			t.Fatalf("RegisterUser failed: %v", err)
		}

		// 测试重复注册
		err = store.RegisterUser(username, password, permission)
		if err == nil {
			t.Error("Should fail when registering existing user")
		}
	})

	// 2. LoginUser
	t.Run("LoginUser", func(t *testing.T) {
		// 成功登录
		perm, err := store.LoginUser(username, password)
		if err != nil {
			t.Fatalf("LoginUser failed: %v", err)
		}
		if perm != permission {
			t.Errorf("Permission mismatch. Got %d, want %d", perm, permission)
		}

		// 密码错误
		_, err = store.LoginUser(username, password+"wrong")
		if err == nil {
			t.Error("Should fail with wrong password")
		}

		// 用户不存在
		_, err = store.LoginUser(username+"nonexist", password)
		if err == nil {
			t.Error("Should fail with non-existent user")
		}
	})

	// 3. ChangePassword
	newPassword := "new_" + randomString(8)
	t.Run("ChangePassword", func(t *testing.T) {
		// 旧密码错误
		err := store.ChangePassword(username, "wrong_old", newPassword)
		if err == nil {
			t.Error("Should fail with wrong old password")
		}

		// 修改成功
		err = store.ChangePassword(username, password, newPassword)
		if err != nil {
			t.Fatalf("ChangePassword failed: %v", err)
		}

		// 验证新密码登录
		_, err = store.LoginUser(username, newPassword)
		if err != nil {
			t.Error("Failed to login with new password")
		}

		// 验证旧密码失效
		_, err = store.LoginUser(username, password)
		if err == nil {
			t.Error("Old password should be invalid")
		}
	})
}

// -----------------------------------------------------------------------------
// 性能测试 (Benchmarks)
// -----------------------------------------------------------------------------

// BenchmarkInitDevice 压测设备注册性能
func BenchmarkInitDevice(b *testing.B) {
	if store == nil {
		b.Skip("Store not initialized")
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		uuid := fmt.Sprintf("bench-init-%d-%d", b.N, i)
		meta := generateRandomMeta()
		// 忽略错误，只测性能
		_ = store.InitDevice(uuid, meta)
	}
}

// BenchmarkAppendMetric 压测传感器数据写入性能
func BenchmarkAppendMetric(b *testing.B) {
	if store == nil {
		b.Skip("Store not initialized")
	}

	// 准备一个设备
	uuid := "bench-metric-device-" + randomString(5)
	store.InitDevice(uuid, generateRandomMeta())

	ts := time.Now().Unix()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.AppendMetric(uuid, inter.MetricPoint{Timestamp: ts + int64(i), Value: rand.Float32(), Type: 0})
	}
}

// BenchmarkGetDeviceByToken 压测鉴权性能 (通常是最高频的操作)
func BenchmarkGetDeviceByToken(b *testing.B) {
	if store == nil {
		b.Skip("Store not initialized")
	}

	uuid := "bench-auth-" + randomString(5)
	meta := generateRandomMeta()
	token := meta.Token
	store.InitDevice(uuid, meta)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = store.GetDeviceByToken(token)
	}
}

// BenchmarkQueryMetrics_Parallel 并发查询性能测试
// 模拟场景：多个协程同时查询同一个设备的数据（测试读锁争用和缓存能力）
func BenchmarkQueryMetrics_Parallel(b *testing.B) {
	if store == nil {
		b.Skip("Store not initialized")
	}

	// 1. 准备数据 (Setup)
	uuid := "bench-query-parallel-" + randomString(5)
	store.InitDevice(uuid, generateRandomMeta())

	// 预埋 5000 条数据 (模拟比较大的数据集)
	base := time.Now().Unix()
	for i := 0; i < 5000; i++ {
		// 忽略插入错误，这里只管造数据
		_ = store.AppendMetric(uuid, inter.MetricPoint{Timestamp: base + int64(i), Value: float32(i), Type: 0})
	}

	// 2. 重置计时器，开始测试
	b.ResetTimer()

	// 3. 使用 RunParallel 利用所有 CPU 核心
	b.RunParallel(func(pb *testing.PB) {
		// 注意：查询通常是只读的，不需要每个协程有独立的随机源，
		// 这里我们模拟查询固定的时间段
		start := base
		end := base + 100 // 每次查 100 个点

		for pb.Next() {
			_, _ = store.QueryMetrics(uuid, start, end)
		}
	})
}

// BenchmarkDeviceIngestion_Parallel 并发设备数据接入测试
// 模拟场景：模拟 b.N 个设备同时发起连接，每个设备写入 batchSize 条数据
// 这里的 ns/op 将代表“处理完一个设备的一批数据所需的时间”
func BenchmarkDeviceIngestion_Parallel(b *testing.B) {
	if store == nil {
		b.Skip("Store not initialized")
	}

	const batchSize = 1000 // 每个设备写入 1000 个采样点

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		// ⚠️关键优化：每个协程使用独立的随机源，避免全局锁竞争
		// 如果使用全局 rand.Intn，由于加锁，性能测试结果会严重失真
		src := rand.NewSource(time.Now().UnixNano())
		r := rand.New(src)

		for pb.Next() {
			// 1. 每个 Op 模拟一个新的设备接入
			// 使用纳秒+随机数尽量保证 UUID 不冲突，避免 InitDevice 报错
			uuid := fmt.Sprintf("bench-ingest-%d-%d", time.Now().UnixNano(), r.Int63())

			// 构造元数据
			meta := inter.DeviceMetadata{
				Name:  "BenchDevice",
				Token: fmt.Sprintf("tk-%d", r.Int63()),
				// ... 其他字段简化 ...
			}

			// 2. 初始化设备
			if err := store.InitDevice(uuid, meta); err != nil {
				// 如果极低概率碰撞了，就跳过
				continue
			}

			var point []inter.MetricPoint = make([]inter.MetricPoint, batchSize)
			var tim = time.Now().UnixNano()
			for i := 0; i < batchSize; i++ {
				tim += 1
				point[i] = inter.MetricPoint{Timestamp: tim, Value: 123.45, Type: 0}
			}
			store.BatchAppendMetrics(uuid, point)

		}
	})
}
