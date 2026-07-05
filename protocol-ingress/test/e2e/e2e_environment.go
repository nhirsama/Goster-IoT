package e2e

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	mqttserver "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	ingressv1 "github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/app"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
	"github.com/stretchr/testify/require"
)

// E2EEnvironment 提供完整的 E2E 测试环境
type E2EEnvironment struct {
	t               *testing.T
	tempDir         string
	mqttBroker      *mqttserver.Server
	mqttAddr        string
	protocolApp     *app.App
	coreServiceMock *MockCoreService
	cancel          context.CancelFunc
	mu              sync.Mutex
	ingestedEvents  []*ingressv1.CanonicalDeviceEvent
}

// MockCoreService 模拟 Core API 服务
type MockCoreService struct {
	mu           sync.Mutex
	devices      map[string]*DeviceRecord
	events       []*ingressv1.CanonicalDeviceEvent
	heartbeats   []*ingressv1.ReportHeartbeatRequest
	updates      []*ingressv1.UpdateCommandStatusRequest
	commands     map[string][]*ingressv1.CanonicalCommand
	eventCounter int64
}

type DeviceRecord struct {
	UUID     string
	TenantID string
	Token    string
	Status   ingressv1.AuthStatus
}

func setupE2EEnvironment(t *testing.T) *E2EEnvironment {
	return setupE2EEnvironmentWithHooks(t, nil, nil)
}

func setupE2EEnvironmentWithConfig(t *testing.T, configure func(*config.Config)) *E2EEnvironment {
	return setupE2EEnvironmentWithHooks(t, configure, nil)
}

func setupE2EEnvironmentWithHooks(t *testing.T, configure func(*config.Config), beforeAppStart func(t *testing.T, mqttAddr string)) *E2EEnvironment {
	t.Helper()

	tempDir := t.TempDir()

	// 启动嵌入式 MQTT broker
	mqttAddr := getFreeTCPAddr(t)
	broker := startMQTTBroker(t, mqttAddr)
	if beforeAppStart != nil {
		beforeAppStart(t, mqttAddr)
	}

	// 创建 mock core service
	mockCore := &MockCoreService{
		devices:    make(map[string]*DeviceRecord),
		commands:   make(map[string][]*ingressv1.CanonicalCommand),
		events:     make([]*ingressv1.CanonicalDeviceEvent, 0),
		heartbeats: make([]*ingressv1.ReportHeartbeatRequest, 0),
		updates:    make([]*ingressv1.UpdateCommandStatusRequest, 0),
	}

	// 预注册一些测试设备
	mockCore.registerTestDevice("device-uuid-001", "tenant-test-001", "token-test-001")

	// 配置 protocol-ingress
	cfg := config.Config{
		Service: config.ServiceConfig{
			Name:       "protocol-ingress-e2e",
			Env:        "test",
			InstanceID: "e2e-test-instance",
		},
		Core: config.CoreConfig{
			Endpoint: "http://localhost:9999", // mock 会拦截
		},
		Adapters: config.AdapterConfig{
			CustomTCP: config.CustomTCPConfig{
				Enabled: false, // E2E 测试只测 MQTT
			},
			MQTT: config.MQTTConfig{
				Enabled:       true,
				BrokerURL:     fmt.Sprintf("tcp://%s", mqttAddr),
				ClientID:      "protocol-ingress-e2e",
				Username:      "",
				Password:      "",
				MessageBuffer: 1000,
				SubscribeTopics: []string{
					"zigbee2mqtt/+",
					"zigbee2mqtt/+/availability",
					"goster/v1/+/+/telemetry",
					"goster/v1/+/+/heartbeat",
					"goster/v1/+/+/ack",
				},
				QoS:                  1,
				RPCTimeout:           time.Second,
				DownlinkEnabled:      false,
				DownlinkPollInterval: 100 * time.Millisecond,
				DownlinkMaxBatch:     4,
			},
		},
		Server: config.ServerConfig{
			HTTPAddr: "", // E2E 测试不需要管理服务器
		},
	}
	if configure != nil {
		configure(&cfg)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn, // 减少测试输出
	}))

	// 使用 mock core client
	mockCoreClient := &MockCoreClient{mockCore: mockCore}

	protocolApp := app.New(cfg, logger, app.WithCoreClient(mockCoreClient))

	// 启动 protocol-ingress
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := protocolApp.Run(ctx); err != nil && err != context.Canceled {
			t.Logf("protocol-ingress 运行错误: %v", err)
		}
	}()

	// 等待服务启动
	time.Sleep(1 * time.Second)

	env := &E2EEnvironment{
		t:               t,
		tempDir:         tempDir,
		mqttBroker:      broker,
		mqttAddr:        mqttAddr,
		protocolApp:     protocolApp,
		coreServiceMock: mockCore,
		cancel:          cancel,
		ingestedEvents:  make([]*ingressv1.CanonicalDeviceEvent, 0),
	}

	t.Cleanup(func() {
		env.Teardown()
	})

	return env
}

func (e *E2EEnvironment) Teardown() {
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	if e.mqttBroker != nil {
		_ = e.mqttBroker.Close()
		e.mqttBroker = nil
	}
}

func (e *E2EEnvironment) connectMQTTClient(t *testing.T, clientID string) mqtt.Client {
	t.Helper()

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", e.mqttAddr))
	opts.SetClientID(clientID)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetConnectTimeout(3 * time.Second)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	require.True(t, token.WaitTimeout(5*time.Second), "MQTT 连接超时")
	require.NoError(t, token.Error(), "MQTT 连接失败")

	return client
}

func (e *E2EEnvironment) registerDevice(t *testing.T, tenantID, uuid string) string {
	t.Helper()
	token := fmt.Sprintf("token-%s", uuid)
	e.coreServiceMock.registerTestDevice(uuid, tenantID, token)
	return token
}

func (e *E2EEnvironment) getIngestedEvents(t *testing.T, minCount int) []*ingressv1.CanonicalDeviceEvent {
	t.Helper()

	// 等待事件到达
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		e.coreServiceMock.mu.Lock()
		count := len(e.coreServiceMock.events)
		e.coreServiceMock.mu.Unlock()

		if count >= minCount {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	e.coreServiceMock.mu.Lock()
	defer e.coreServiceMock.mu.Unlock()
	require.GreaterOrEqual(t, len(e.coreServiceMock.events), minCount, "Core 未收到期望数量的事件")

	// 返回最近的事件
	events := make([]*ingressv1.CanonicalDeviceEvent, 0)
	startIdx := len(e.coreServiceMock.events) - minCount
	if startIdx < 0 {
		startIdx = 0
	}
	events = append(events, e.coreServiceMock.events[startIdx:]...)

	return events
}

func (e *E2EEnvironment) getIngestedEventsSince(t *testing.T, before int64, expectedNew int) []*ingressv1.CanonicalDeviceEvent {
	t.Helper()
	target := before + int64(expectedNew)
	require.Eventually(t, func() bool {
		return e.getIngestedEventCount(t) >= target
	}, 5*time.Second, 50*time.Millisecond, "Core 未收到新的事件")

	e.coreServiceMock.mu.Lock()
	defer e.coreServiceMock.mu.Unlock()
	startIdx := int(before)
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > len(e.coreServiceMock.events) {
		startIdx = len(e.coreServiceMock.events)
	}
	out := make([]*ingressv1.CanonicalDeviceEvent, len(e.coreServiceMock.events[startIdx:]))
	copy(out, e.coreServiceMock.events[startIdx:])
	return out
}

func (e *E2EEnvironment) getIngestedEventCount(t *testing.T) int64 {
	t.Helper()
	e.coreServiceMock.mu.Lock()
	defer e.coreServiceMock.mu.Unlock()
	return int64(len(e.coreServiceMock.events))
}

func (e *E2EEnvironment) enqueueCommand(t *testing.T, uuid string, cmd *ingressv1.CanonicalCommand) {
	t.Helper()
	e.coreServiceMock.mu.Lock()
	defer e.coreServiceMock.mu.Unlock()
	e.coreServiceMock.commands[uuid] = append(e.coreServiceMock.commands[uuid], cmd)
}

func (e *E2EEnvironment) getHeartbeats(t *testing.T, minCount int) []*ingressv1.ReportHeartbeatRequest {
	t.Helper()
	require.Eventually(t, func() bool {
		e.coreServiceMock.mu.Lock()
		defer e.coreServiceMock.mu.Unlock()
		return len(e.coreServiceMock.heartbeats) >= minCount
	}, 5*time.Second, 50*time.Millisecond, "Core 未收到期望数量的 heartbeat")

	e.coreServiceMock.mu.Lock()
	defer e.coreServiceMock.mu.Unlock()
	out := make([]*ingressv1.ReportHeartbeatRequest, len(e.coreServiceMock.heartbeats))
	copy(out, e.coreServiceMock.heartbeats)
	return out
}

func (e *E2EEnvironment) getCommandUpdates(t *testing.T, minCount int) []*ingressv1.UpdateCommandStatusRequest {
	t.Helper()
	require.Eventually(t, func() bool {
		e.coreServiceMock.mu.Lock()
		defer e.coreServiceMock.mu.Unlock()
		return len(e.coreServiceMock.updates) >= minCount
	}, 5*time.Second, 50*time.Millisecond, "Core 未收到期望数量的 command status update")

	e.coreServiceMock.mu.Lock()
	defer e.coreServiceMock.mu.Unlock()
	out := make([]*ingressv1.UpdateCommandStatusRequest, len(e.coreServiceMock.updates))
	copy(out, e.coreServiceMock.updates)
	return out
}

func (e *E2EEnvironment) publishDownlinkCommand(t *testing.T, deviceName string, payload map[string]interface{}) {
	t.Helper()
	// 这里模拟通过 downlink topic 发送命令
	// 实际实现需要根据你的下行命令机制
	t.Logf("模拟下发命令到设备 %s: %+v", deviceName, payload)
}

func (m *MockCoreService) registerTestDevice(uuid, tenantID, token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.devices[token] = &DeviceRecord{
		UUID:     uuid,
		TenantID: tenantID,
		Token:    token,
		Status:   ingressv1.AuthStatus_AUTH_STATUS_ACCEPTED,
	}
}

func (m *MockCoreService) AuthenticateDevice(ctx context.Context, req *ingressv1.AuthenticateDeviceRequest) (*ingressv1.AuthenticateDeviceResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 从 credentials 中提取 token
	var token string
	for _, cred := range req.Credentials {
		if cred.Type == "token" {
			token = cred.Value
			break
		}
	}

	if device, ok := m.devices[token]; ok {
		return &ingressv1.AuthenticateDeviceResponse{
			Status:   device.Status,
			Uuid:     device.UUID,
			TenantId: device.TenantID,
		}, nil
	}

	return &ingressv1.AuthenticateDeviceResponse{
		Status: ingressv1.AuthStatus_AUTH_STATUS_REJECTED,
		Reason: "device not found",
	}, nil
}

func (m *MockCoreService) RegisterDevice(ctx context.Context, req *ingressv1.RegisterDeviceRequest) (*ingressv1.RegisterDeviceResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	uuid := req.Device.Uuid
	if uuid == "" {
		uuid = fmt.Sprintf("device-%d", time.Now().UnixNano())
	}

	token := fmt.Sprintf("token-%s", uuid)
	m.devices[token] = &DeviceRecord{
		UUID:     uuid,
		TenantID: req.Context.TenantHint,
		Token:    token,
		Status:   ingressv1.AuthStatus_AUTH_STATUS_ACCEPTED,
	}

	return &ingressv1.RegisterDeviceResponse{
		Status:   ingressv1.RegistrationStatus_REGISTRATION_STATUS_ACCEPTED,
		Uuid:     uuid,
		TenantId: req.Context.TenantHint,
		Credential: &ingressv1.Credential{
			Type:  "token",
			Value: token,
		},
	}, nil
}

func (m *MockCoreService) ReportHeartbeat(ctx context.Context, req *ingressv1.ReportHeartbeatRequest) (*ingressv1.ReportHeartbeatResponse, error) {
	m.mu.Lock()
	m.heartbeats = append(m.heartbeats, req)
	m.mu.Unlock()

	return &ingressv1.ReportHeartbeatResponse{
		Uuid:         req.Uuid,
		TenantId:     req.Context.TenantHint,
		Availability: ingressv1.DeviceAvailability_DEVICE_AVAILABILITY_ONLINE,
	}, nil
}

func (m *MockCoreService) IngestEvents(ctx context.Context, req *ingressv1.IngestEventsRequest) (*ingressv1.IngestEventsResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = append(m.events, req.Events...)
	m.eventCounter += int64(len(req.Events))

	results := make([]*ingressv1.EventIngestResult, len(req.Events))
	for i, event := range req.Events {
		results[i] = &ingressv1.EventIngestResult{
			EventId:  event.EventId,
			Success:  true,
			Uuid:     event.PrimaryIdentity.Value,
			TenantId: event.Context.TenantHint,
		}
	}

	return &ingressv1.IngestEventsResponse{
		Results: results,
	}, nil
}

func (m *MockCoreService) PullCommands(ctx context.Context, req *ingressv1.PullCommandsRequest) (*ingressv1.PullCommandsResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	uuid := req.Uuid
	if cmds, ok := m.commands[uuid]; ok {
		limit := int(req.GetMaxCount())
		if limit <= 0 || limit > len(cmds) {
			limit = len(cmds)
		}
		out := append([]*ingressv1.CanonicalCommand(nil), cmds[:limit]...)
		if limit == len(cmds) {
			delete(m.commands, uuid) // 拉取后清空
		} else {
			m.commands[uuid] = cmds[limit:]
		}
		return &ingressv1.PullCommandsResponse{
			Commands: out,
		}, nil
	}

	return &ingressv1.PullCommandsResponse{
		Commands: []*ingressv1.CanonicalCommand{},
	}, nil
}

func (m *MockCoreService) UpdateCommandStatus(ctx context.Context, req *ingressv1.UpdateCommandStatusRequest) (*ingressv1.UpdateCommandStatusResponse, error) {
	m.mu.Lock()
	m.updates = append(m.updates, req)
	m.mu.Unlock()

	return &ingressv1.UpdateCommandStatusResponse{
		Success: true,
		Status:  req.Status,
	}, nil
}

// MockCoreClient 实现 coreclient.Client 接口
type MockCoreClient struct {
	mockCore *MockCoreService
}

func (m *MockCoreClient) AuthenticateDevice(ctx context.Context, req *ingressv1.AuthenticateDeviceRequest) (*ingressv1.AuthenticateDeviceResponse, error) {
	return m.mockCore.AuthenticateDevice(ctx, req)
}

func (m *MockCoreClient) RegisterDevice(ctx context.Context, req *ingressv1.RegisterDeviceRequest) (*ingressv1.RegisterDeviceResponse, error) {
	return m.mockCore.RegisterDevice(ctx, req)
}

func (m *MockCoreClient) ReportHeartbeat(ctx context.Context, req *ingressv1.ReportHeartbeatRequest) (*ingressv1.ReportHeartbeatResponse, error) {
	return m.mockCore.ReportHeartbeat(ctx, req)
}

func (m *MockCoreClient) IngestEvents(ctx context.Context, req *ingressv1.IngestEventsRequest) (*ingressv1.IngestEventsResponse, error) {
	return m.mockCore.IngestEvents(ctx, req)
}

func (m *MockCoreClient) PullCommands(ctx context.Context, req *ingressv1.PullCommandsRequest) (*ingressv1.PullCommandsResponse, error) {
	return m.mockCore.PullCommands(ctx, req)
}

func (m *MockCoreClient) UpdateCommandStatus(ctx context.Context, req *ingressv1.UpdateCommandStatusRequest) (*ingressv1.UpdateCommandStatusResponse, error) {
	return m.mockCore.UpdateCommandStatus(ctx, req)
}

func startMQTTBroker(t *testing.T, addr string) *mqttserver.Server {
	t.Helper()

	broker := mqttserver.New(&mqttserver.Options{
		InlineClient: true,
	})

	// 允许匿名连接（测试环境）
	_ = broker.AddHook(new(auth.AllowHook), nil)

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "e2e-tcp",
		Address: addr,
	})

	err := broker.AddListener(tcp)
	require.NoError(t, err, "添加 MQTT listener 失败")

	go func() {
		err := broker.Serve()
		if err != nil {
			t.Logf("MQTT broker 错误: %v", err)
		}
	}()

	// 等待 broker 启动
	time.Sleep(500 * time.Millisecond)

	return broker
}

func getFreeTCPAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	_ = l.Close()

	return addr
}
