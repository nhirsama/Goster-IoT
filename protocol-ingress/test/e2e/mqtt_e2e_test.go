package e2e

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMQTTZigbee2MQTTRealWorldScenario 模拟真实 Zigbee2MQTT 场景
func TestMQTTZigbee2MQTTRealWorldScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	env := setupE2EEnvironment(t)
	defer env.Teardown()

	client := env.connectMQTTClient(t, "test-z2m-client")
	defer client.Disconnect(250)

	// 场景 1: 温湿度传感器上报数据
	t.Run("temperature_humidity_sensor", func(t *testing.T) {
		payload := map[string]interface{}{
			"temperature": 23.5,
			"humidity":    65.2,
			"battery":     95,
			"voltage":     3000,
			"pressure":    1012.7,
			"linkquality": 120,
			"last_seen":   time.Now().Format(time.RFC3339),
		}

		publishAndVerify(t, client, env, "zigbee2mqtt/living_room_sensor", payload, func(events []*ingressv1.CanonicalDeviceEvent) {
			require.Len(t, events, 1)
			event := events[0]

			assert.Equal(t, ingressv1.EventType_EVENT_TYPE_TELEMETRY, event.EventType)
			assert.Equal(t, "zigbee2mqtt", event.Context.ProtocolName)
			assert.Equal(t, "living_room_sensor", event.PrimaryIdentity.Value)

			// 验证 metrics
			assert.GreaterOrEqual(t, len(event.Metrics), 4)
			assertMetric(t, event.Metrics, "temperature", 23.5)
			assertMetric(t, event.Metrics, "humidity", 65.2)
			assertMetric(t, event.Metrics, "battery", 95.0)
			assertMetric(t, event.Metrics, "pressure", 1012.7)
			assertMetric(t, event.Metrics, "linkquality", 120.0)
		})
	})

	// 场景 2: 门磁传感器（contact sensor）
	t.Run("contact_sensor", func(t *testing.T) {
		payload := map[string]interface{}{
			"contact":     false, // 门打开
			"battery":     88,
			"voltage":     2950,
			"tamper":      false,
			"linkquality": 105,
		}

		publishAndVerify(t, client, env, "zigbee2mqtt/front_door_sensor", payload, func(events []*ingressv1.CanonicalDeviceEvent) {
			require.Len(t, events, 1)
			event := events[0]

			// contact 是状态，不是 metric
			assert.GreaterOrEqual(t, len(event.States), 1)
			contactState := findState(event.States, "contact")
			require.NotNil(t, contactState)
			assert.False(t, contactState.Value.GetBoolValue())

			// tamper 也是状态
			tamperState := findState(event.States, "tamper")
			require.NotNil(t, tamperState)
			assert.False(t, tamperState.Value.GetBoolValue())
		})
	})

	// 场景 3: 人体传感器（occupancy sensor）
	t.Run("occupancy_sensor", func(t *testing.T) {
		payload := map[string]interface{}{
			"occupancy":       true,
			"illuminance":     450,
			"illuminance_lux": 450,
			"battery":         92,
			"linkquality":     115,
		}

		publishAndVerify(t, client, env, "zigbee2mqtt/bedroom_motion", payload, func(events []*ingressv1.CanonicalDeviceEvent) {
			require.Len(t, events, 1)
			event := events[0]

			// occupancy 是状态
			occupancyState := findState(event.States, "occupancy")
			require.NotNil(t, occupancyState)
			assert.True(t, occupancyState.Value.GetBoolValue())

			// illuminance 是 metric
			assertMetric(t, event.Metrics, "illuminance", 450.0)
		})
	})

	// 场景 4: 智能插座状态上报
	t.Run("smart_plug_status", func(t *testing.T) {
		// 上报当前状态
		statusPayload := map[string]interface{}{
			"state":       "ON",
			"power":       25.5,  // 瓦特
			"current":     0.21,  // 安培
			"energy":      1.34,  // 千瓦时
			"voltage":     230.1, // 伏特
			"linkquality": 125,
		}

		publishAndVerify(t, client, env, "zigbee2mqtt/kitchen_plug", statusPayload, func(events []*ingressv1.CanonicalDeviceEvent) {
			require.Len(t, events, 1)
			event := events[0]

			// state 是状态
			state := findState(event.States, "state")
			require.NotNil(t, state)
			assert.Equal(t, "ON", state.Value.GetStringValue())

			// power/voltage 是 metrics
			assertMetric(t, event.Metrics, "power", 25.5)
			assertMetric(t, event.Metrics, "current", 0.21)
			assertMetric(t, event.Metrics, "energy", 1.34)
			assertMetric(t, event.Metrics, "voltage", 230.1)
		})
	})

	// 场景 5: 多传感器设备
	t.Run("multi_sensor_device", func(t *testing.T) {
		payload := map[string]interface{}{
			"temperature": 22.8,
			"humidity":    58.3,
			"pressure":    1013.25, // 气压
			"occupancy":   false,
			"illuminance": 120,
			"battery":     89,
			"linkquality": 110,
		}

		publishAndVerify(t, client, env, "zigbee2mqtt/office_multi_sensor", payload, func(events []*ingressv1.CanonicalDeviceEvent) {
			require.Len(t, events, 1)
			event := events[0]

			// 验证核心 metrics（temperature, humidity, battery 通常都支持）
			assertMetric(t, event.Metrics, "temperature", 22.8)
			assertMetric(t, event.Metrics, "humidity", 58.3)
			assertMetric(t, event.Metrics, "battery", 89.0)
			assertMetric(t, event.Metrics, "pressure", 1013.25)

			// occupancy 是状态
			occupancyState := findState(event.States, "occupancy")
			require.NotNil(t, occupancyState)
			assert.False(t, occupancyState.Value.GetBoolValue())

			t.Logf("多传感器设备上报成功，metrics: %d, states: %d", len(event.Metrics), len(event.States))
		})
	})

	// 场景 6: 水浸/烟雾/低电量/遥控动作等离散状态
	t.Run("safety_and_action_states", func(t *testing.T) {
		payload := map[string]interface{}{
			"water_leak":  true,
			"smoke":       false,
			"action":      "single",
			"battery_low": false,
			"linkquality": 88,
		}

		publishAndVerify(t, client, env, "zigbee2mqtt/safety_cluster", payload, func(events []*ingressv1.CanonicalDeviceEvent) {
			require.Len(t, events, 1)
			event := events[0]

			waterLeak := findState(event.States, "water_leak")
			require.NotNil(t, waterLeak)
			assert.True(t, waterLeak.Value.GetBoolValue())

			smoke := findState(event.States, "smoke")
			require.NotNil(t, smoke)
			assert.False(t, smoke.Value.GetBoolValue())

			action := findState(event.States, "action")
			require.NotNil(t, action)
			assert.Equal(t, "single", action.Value.GetStringValue())

			batteryLow := findState(event.States, "battery_low")
			require.NotNil(t, batteryLow)
			assert.False(t, batteryLow.Value.GetBoolValue())
		})
	})

	// 场景 7: 灯具状态（开关、亮度、色温）
	t.Run("light_state", func(t *testing.T) {
		payload := map[string]interface{}{
			"state":       "ON",
			"brightness":  180,
			"color_temp":  370,
			"linkquality": 140,
		}

		publishAndVerify(t, client, env, "zigbee2mqtt/desk_light", payload, func(events []*ingressv1.CanonicalDeviceEvent) {
			require.Len(t, events, 1)
			event := events[0]

			state := findState(event.States, "state")
			require.NotNil(t, state)
			assert.Equal(t, "ON", state.Value.GetStringValue())

			brightness := findState(event.States, "brightness")
			require.NotNil(t, brightness)
			assert.InDelta(t, 180.0, brightness.Value.GetNumberValue(), 0.01)

			colorTemp := findState(event.States, "color_temp")
			require.NotNil(t, colorTemp)
			assert.InDelta(t, 370.0, colorTemp.Value.GetNumberValue(), 0.01)
		})
	})
}

// TestMQTTGosterProtocolScenario 测试 Goster 自有协议
func TestMQTTGosterProtocolScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	env := setupE2EEnvironment(t)
	defer env.Teardown()

	client := env.connectMQTTClient(t, "test-goster-client")
	defer client.Disconnect(250)

	tenantID := "tenant-test-001"
	deviceUUID := "device-uuid-001"
	deviceToken := env.registerDevice(t, tenantID, deviceUUID)

	// 场景 1: 设备上报遥测数据
	t.Run("goster_telemetry", func(t *testing.T) {
		payload := map[string]interface{}{
			"token":       deviceToken,
			"temperature": 25.5,
			"humidity":    60.0,
			"ts":          time.Now().UnixMilli(),
		}

		topic := fmt.Sprintf("goster/v1/%s/%s/telemetry", tenantID, deviceUUID)
		publishAndVerify(t, client, env, topic, payload, func(events []*ingressv1.CanonicalDeviceEvent) {
			require.Len(t, events, 1)
			event := events[0]

			assert.Equal(t, ingressv1.EventType_EVENT_TYPE_TELEMETRY, event.EventType)
			assert.Equal(t, deviceUUID, event.PrimaryIdentity.Value)
			assert.Equal(t, tenantID, event.Context.TenantHint)

			assertMetric(t, event.Metrics, "temperature", 25.5)
			assertMetric(t, event.Metrics, "humidity", 60.0)

			t.Log("Goster 遥测数据上报成功")
		})
	})

	// 场景 2: 设备心跳上报
	t.Run("goster_heartbeat", func(t *testing.T) {
		payload := map[string]interface{}{
			"token":        deviceToken,
			"availability": "offline",
			"ts":           time.Now().UnixMilli(),
		}
		topic := fmt.Sprintf("goster/v1/%s/%s/heartbeat", tenantID, deviceUUID)
		publishJSON(t, client, topic, payload, 1, false)

		heartbeats := env.getHeartbeats(t, 1)
		heartbeat := heartbeats[len(heartbeats)-1]
		assert.Equal(t, deviceUUID, heartbeat.GetUuid())
		assert.Equal(t, ingressv1.DeviceAvailability_DEVICE_AVAILABILITY_OFFLINE, heartbeat.GetAvailability())
		assert.Equal(t, tenantID, heartbeat.GetContext().GetTenantHint())
	})

	// 场景 3: 设备命令 ACK/失败回执
	t.Run("goster_command_ack", func(t *testing.T) {
		payload := map[string]interface{}{
			"token":        deviceToken,
			"command_id":   42,
			"command_uuid": "cmd-42",
			"status":       "acked",
			"operation":    "set",
		}
		topic := fmt.Sprintf("goster/v1/%s/%s/ack", tenantID, deviceUUID)
		publishJSON(t, client, topic, payload, 1, false)

		updates := env.getCommandUpdates(t, 1)
		update := updates[len(updates)-1]
		assert.Equal(t, int64(42), update.GetCommandId())
		assert.Equal(t, "cmd-42", update.GetCommandUuid())
		assert.Equal(t, ingressv1.CommandStatus_COMMAND_STATUS_ACKED, update.GetStatus())
		assert.Equal(t, deviceUUID, update.GetUuid())
		assert.Equal(t, "set", update.GetOperation())
	})
}

// TestMQTTHighThroughputScenario 测试高吞吐量场景
func TestMQTTHighThroughputScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	env := setupE2EEnvironment(t)
	defer env.Teardown()

	client := env.connectMQTTClient(t, "test-throughput-client")
	defer client.Disconnect(250)

	beforeCount := env.getIngestedEventCount(t)

	// 模拟 100 个设备同时上报数据
	numDevices := 100
	numMessagesPerDevice := 10

	var wg sync.WaitGroup
	errors := make(chan error, numDevices)

	start := time.Now()

	for i := 0; i < numDevices; i++ {
		wg.Add(1)
		go func(deviceID int) {
			defer wg.Done()

			deviceName := fmt.Sprintf("load_test_device_%d", deviceID)
			for j := 0; j < numMessagesPerDevice; j++ {
				payload := map[string]interface{}{
					"temperature": 20.0 + float64(deviceID%10),
					"humidity":    50.0 + float64(j%20),
					"battery":     80 + (deviceID % 20),
					"linkquality": 100 + (deviceID % 55),
					"sequence":    j,
				}

				payloadBytes, err := json.Marshal(payload)
				if err != nil {
					errors <- err
					return
				}

				topic := fmt.Sprintf("zigbee2mqtt/%s", deviceName)
				token := client.Publish(topic, 1, false, payloadBytes)
				if !token.WaitTimeout(5 * time.Second) {
					errors <- fmt.Errorf("设备 %d 消息 %d 发布超时", deviceID, j)
					return
				}
				if token.Error() != nil {
					errors <- token.Error()
					return
				}

				// 小延迟避免过载
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// 检查错误
	for err := range errors {
		t.Errorf("吞吐量测试错误: %v", err)
	}

	elapsed := time.Since(start)
	totalMessages := numDevices * numMessagesPerDevice
	throughput := float64(totalMessages) / elapsed.Seconds()

	t.Logf("吞吐量测试完成: %d 条消息在 %v 内发送，吞吐量 %.2f msg/s", totalMessages, elapsed, throughput)

	// QoS 1 场景下应收到全部消息
	events := env.getIngestedEventsSince(t, beforeCount, totalMessages)
	t.Logf("Core 新接收到 %d 条事件", len(events))
	require.GreaterOrEqual(t, len(events), totalMessages, "QoS 1 高吞吐场景应完整接收消息")
}

// TestMQTTRetainedMessageScenario 测试 retained 消息
func TestMQTTRetainedMessageScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	payload := map[string]interface{}{
		"temperature": 24.5,
		"humidity":    55.0,
		"linkquality": 130,
	}

	// 先向 broker 写入 retained 消息，再启动 protocol-ingress，验证 adapter 订阅时能处理 retained 初始状态。
	env := setupE2EEnvironmentWithHooks(t, nil, func(t *testing.T, mqttAddr string) {
		prePublishRetainedJSON(t, mqttAddr, "zigbee2mqtt/persistent_sensor", payload)
	})
	defer env.Teardown()

	events := env.getIngestedEvents(t, 1)
	event := events[len(events)-1]

	assert.Equal(t, "persistent_sensor", event.PrimaryIdentity.Value)
	assert.Equal(t, "true", event.Context.Labels["mqtt_retained"])
	assertMetric(t, event.Metrics, "temperature", 24.5)
	assertMetric(t, event.Metrics, "humidity", 55.0)
}

// TestMQTTZigbee2MQTTAvailabilityAndRawFallback 测试 availability 子 topic 与非 JSON/坏 JSON payload。
func TestMQTTZigbee2MQTTAvailabilityAndRawFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	env := setupE2EEnvironment(t)
	defer env.Teardown()

	client := env.connectMQTTClient(t, "test-availability-client")
	defer client.Disconnect(250)

	publishRaw(t, client, "zigbee2mqtt/front_door_sensor/availability", []byte("offline"), 1, false)
	availabilityEvents := env.getIngestedEvents(t, 1)
	availability := availabilityEvents[len(availabilityEvents)-1]
	assert.Equal(t, ingressv1.EventType_EVENT_TYPE_AVAILABILITY, availability.EventType)
	assert.Equal(t, ingressv1.DeviceAvailability_DEVICE_AVAILABILITY_OFFLINE, availability.Availability)
	assert.Equal(t, "front_door_sensor", availability.PrimaryIdentity.Value)

	before := env.getIngestedEventCount(t)
	publishRaw(t, client, "zigbee2mqtt/bad_json_sensor", []byte(`{"temperature":`), 1, false)
	rawEvents := env.getIngestedEventsSince(t, before, 1)
	raw := rawEvents[len(rawEvents)-1]
	assert.Equal(t, ingressv1.EventType_EVENT_TYPE_DEVICE_EVENT, raw.EventType)
	assert.Equal(t, `{"temperature":`, string(raw.Raw.Body))
	assert.Empty(t, raw.Metrics)
	assert.Empty(t, raw.States)
}

// TestMQTTAuthenticationRejectsInvalidToken 验证无效 token 不会进入 Core IngestEvents。
func TestMQTTAuthenticationRejectsInvalidToken(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	env := setupE2EEnvironment(t)
	defer env.Teardown()

	client := env.connectMQTTClient(t, "test-invalid-token-client")
	defer client.Disconnect(250)

	before := env.getIngestedEventCount(t)
	publishJSON(t, client, "goster/v1/tenant-test-001/device-uuid-001/telemetry", map[string]interface{}{
		"token":       "invalid-token",
		"temperature": 20.5,
	}, 1, false)

	require.Never(t, func() bool {
		return env.getIngestedEventCount(t) > before
	}, time.Second, 50*time.Millisecond, "无效 token 的消息不应被 ingest")
}

// TestMQTTDownlinkCommandScenario 验证设备上行后，adapter 能拉取 Core 命令并发布到 MQTT 下行 topic。
func TestMQTTDownlinkCommandScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	env := setupE2EEnvironmentWithConfig(t, func(cfg *config.Config) {
		cfg.Adapters.MQTT.DownlinkEnabled = true
		cfg.Adapters.MQTT.DownlinkPollInterval = 100 * time.Millisecond
		cfg.Adapters.MQTT.DownlinkMaxBatch = 2
	})
	defer env.Teardown()

	client := env.connectMQTTClient(t, "test-downlink-client")
	defer client.Disconnect(250)

	downlinkTopic := "goster/v1/tenant-test-001/device-uuid-001/downlink"
	downlinkPayloads := make(chan []byte, 1)
	token := client.Subscribe(downlinkTopic, 1, func(c mqtt.Client, m mqtt.Message) {
		downlinkPayloads <- append([]byte(nil), m.Payload()...)
	})
	require.True(t, token.WaitTimeout(2*time.Second))
	require.NoError(t, token.Error())

	env.enqueueCommand(t, "device-uuid-001", &ingressv1.CanonicalCommand{
		CommandId:      1001,
		CommandUuid:    "cmd-downlink-1001",
		TenantId:       "tenant-test-001",
		Uuid:           "device-uuid-001",
		Operation:      "set",
		TargetIdentity: &ingressv1.DeviceIdentity{Type: "uuid", Value: "device-uuid-001"},
		Payload: &ingressv1.RawPayload{
			ContentType: "application/json",
			Text:        `{"state":"OFF","brightness":0}`,
		},
	})

	publishJSON(t, client, "goster/v1/tenant-test-001/device-uuid-001/telemetry", map[string]interface{}{
		"token":       "token-test-001",
		"temperature": 21.5,
	}, 1, false)
	env.getIngestedEvents(t, 1)

	select {
	case payload := <-downlinkPayloads:
		require.JSONEq(t, `{"state":"OFF","brightness":0}`, string(payload))
	case <-time.After(5 * time.Second):
		t.Fatal("未收到 MQTT 下行命令")
	}

	updates := env.getCommandUpdates(t, 1)
	update := updates[len(updates)-1]
	assert.Equal(t, int64(1001), update.GetCommandId())
	assert.Equal(t, "cmd-downlink-1001", update.GetCommandUuid())
	assert.Equal(t, ingressv1.CommandStatus_COMMAND_STATUS_SENT, update.GetStatus())
}

// Helper functions

func assertMetric(t *testing.T, metrics []*ingressv1.MetricPoint, name string, expectedValue float64) {
	t.Helper()
	for _, m := range metrics {
		if m.Name == name {
			assert.InDelta(t, expectedValue, m.Value.GetNumberValue(), 0.01, "metric %s 值不匹配", name)
			return
		}
	}
	t.Errorf("未找到 metric: %s", name)
}

func findState(states []*ingressv1.StatePoint, name string) *ingressv1.StatePoint {
	for _, s := range states {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func publishAndVerify(t *testing.T, client mqtt.Client, env *E2EEnvironment, topic string, payload map[string]interface{}, verify func([]*ingressv1.CanonicalDeviceEvent)) {
	t.Helper()

	before := env.getIngestedEventCount(t)
	publishJSON(t, client, topic, payload, 1, false)

	// 从 core 获取事件
	events := env.getIngestedEventsSince(t, before, 1)
	verify(events)
}

func publishJSON(t *testing.T, client mqtt.Client, topic string, payload map[string]interface{}, qos byte, retained bool) {
	t.Helper()
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)
	publishRaw(t, client, topic, payloadBytes, qos, retained)
}

func publishRaw(t *testing.T, client mqtt.Client, topic string, payload []byte, qos byte, retained bool) {
	t.Helper()
	token := client.Publish(topic, qos, retained, payload)
	require.True(t, token.WaitTimeout(2*time.Second), "MQTT publish 超时: topic=%s", topic)
	require.NoError(t, token.Error(), "MQTT publish 失败: topic=%s", topic)
}

func prePublishRetainedJSON(t *testing.T, mqttAddr, topic string, payload map[string]interface{}) {
	t.Helper()
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", mqttAddr))
	opts.SetClientID("test-retained-prepublisher")
	opts.SetCleanSession(true)
	opts.SetConnectTimeout(3 * time.Second)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	require.True(t, token.WaitTimeout(5*time.Second), "MQTT retained 预发布连接超时")
	require.NoError(t, token.Error(), "MQTT retained 预发布连接失败")
	defer client.Disconnect(250)

	publishJSON(t, client, topic, payload, 1, true)
}
