package e2e

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestMQTTBasicConnectivity 测试基本的 MQTT 连接和消息传递
func TestMQTTBasicConnectivity(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	env := setupE2EEnvironment(t)
	defer env.Teardown()

	// 测试 MQTT 客户端连接
	client := env.connectMQTTClient(t, "test-basic-client")
	defer client.Disconnect(250)

	// 发布一条简单的消息
	payload := map[string]interface{}{
		"temperature": 25.5,
		"humidity":    60.0,
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	token := client.Publish("zigbee2mqtt/test_sensor", 1, false, payloadBytes)
	require.True(t, token.WaitTimeout(2*time.Second))
	require.NoError(t, token.Error())

	t.Log("消息发布成功")

	// 等待消息被处理
	time.Sleep(2 * time.Second)

	// 验证 core 收到了事件
	count := env.getIngestedEventCount(t)
	t.Logf("Core 接收到 %d 条事件", count)

	require.Greater(t, count, int64(0), "消息应成功从 MQTT 传递到 Core")
	t.Log("✅ E2E 测试通过：消息成功从 MQTT 传递到 Core")
}
