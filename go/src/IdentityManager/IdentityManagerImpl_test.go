package IdentityManager

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/DataStore"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/stretchr/testify/assert"
)

// 随机生成器：产生模拟的 SN 和 MAC
func generateRandomMeta(r *rand.Rand) inter.DeviceMetadata {
	// 随机 SN: 如 SN-7A2B-9F3C...
	sn := fmt.Sprintf("SN-%04X-%04X-%04X", r.Intn(0xFFFF), r.Intn(0xFFFF), r.Intn(0xFFFF))

	// 随机 MAC: 00:00:00:00:00:00 格式
	mac := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		r.Intn(256), r.Intn(256), r.Intn(256), r.Intn(256), r.Intn(256), r.Intn(256))

	return inter.DeviceMetadata{SerialNumber: sn, MACAddress: mac}
}

// 校验是否为合法的 Hex 字符串（防止出现你之前的乱码报错）
func isHexString(s string) bool {
	_, err := hex.DecodeString(s)
	// 或者简单的正则：^[0-9a-fA-F]+$
	matched, _ := regexp.MatchString(`^[0-9a-fA-F-]+$`, s) // 允许 UUID 里的横杠
	return err == nil || matched
}

func TestIdentityManager_DeepRandom(t *testing.T) {
	ds, _ := DataStore.NewLocalStore("./test_temp_data")
	mgr := IdentityManager{DataStore: ds}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 1. 批量生成 200 个完全随机的设备
	const deviceCount = 200
	type record struct {
		meta  inter.DeviceMetadata
		uuid  string
		token string
	}
	devices := make([]record, deviceCount)

	t.Run("Batch_Random_Registration", func(t *testing.T) {
		for i := 0; i < deviceCount; i++ {
			meta := generateRandomMeta(rng)
			uuid := mgr.GenerateUUID(meta)

			// ！！！核心校验：确保生成的 UUID 不是乱码二进制！！！
			assert.True(t, isHexString(uuid), "生成的 UUID [%v] 包含非 Hex 字符，会导致文件系统报错", uuid)

			token, err := mgr.RegisterDevice(uuid, meta)
			if !assert.NoError(t, err, "注册设备失败，可能是目录名乱码导致 invalid argument") {
				t.FailNow()
			}

			devices[i] = record{meta, uuid, token}
		}
	})

	t.Run("Cross_Validation_And_Concurrency", func(t *testing.T) {
		// 模拟高频并发访问这 200 个随机设备
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ { // 50 个并发协程
			wg.Add(1)
			go func() {
				defer wg.Done()
				innerRng := rand.New(rand.NewSource(time.Now().UnixNano()))
				for j := 0; j < 100; j++ {
					// 随机选一个已生成的设备进行鉴权
					target := devices[innerRng.Intn(deviceCount)]
					authUUID, err := mgr.Authenticate(target.token)

					assert.NoError(t, err)
					assert.Equal(t, target.uuid, authUUID)
				}
			}()
		}
		wg.Wait()
	})
}

func BenchmarkIdentity_HighEntropy(b *testing.B) {
	ds, _ := DataStore.NewLocalStore("./bench_data")
	mgr := IdentityManager{DataStore: ds}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 预填充 1000 个随机设备，模拟真实数据库规模
	var tokens []string
	for i := 0; i < 1000; i++ {
		meta := generateRandomMeta(rng)
		uid := mgr.GenerateUUID(meta)
		tok, _ := mgr.RegisterDevice(uid, meta)
		tokens = append(tokens, tok)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			// 随机抽取 Token 鉴权
			t := tokens[r.Intn(len(tokens))]
			_, _ = mgr.Authenticate(t)
		}
	})
}
