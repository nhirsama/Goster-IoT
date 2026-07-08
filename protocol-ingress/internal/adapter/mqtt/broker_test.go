package mqtt

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	ingressv1 "github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/normalizer"
)

type embeddedBrokerFakeDevice struct {
	uuid     string
	tenantID string
}

type embeddedBrokerFakeCore struct {
	mu      sync.Mutex
	devices map[string]embeddedBrokerFakeDevice
	events  []*ingressv1.CanonicalDeviceEvent
}

func newEmbeddedBrokerFakeCore() *embeddedBrokerFakeCore {
	return &embeddedBrokerFakeCore{
		devices: map[string]embeddedBrokerFakeDevice{
			"token-dev-1": {uuid: "dev-1", tenantID: "tenant-a"},
			"token-dev-2": {uuid: "dev-2", tenantID: "tenant-a"},
		},
	}
}

func (f *embeddedBrokerFakeCore) AuthenticateDevice(_ context.Context, req *ingressv1.AuthenticateDeviceRequest) (*ingressv1.AuthenticateDeviceResponse, error) {
	token := ""
	for _, cred := range req.GetCredentials() {
		if cred.GetType() == "token" {
			token = cred.GetValue()
			break
		}
	}
	f.mu.Lock()
	device, ok := f.devices[token]
	f.mu.Unlock()
	if !ok {
		return &ingressv1.AuthenticateDeviceResponse{Status: ingressv1.AuthStatus_AUTH_STATUS_REJECTED, Reason: "invalid token"}, nil
	}
	return &ingressv1.AuthenticateDeviceResponse{
		Status:   ingressv1.AuthStatus_AUTH_STATUS_ACCEPTED,
		Uuid:     device.uuid,
		TenantId: device.tenantID,
	}, nil
}

func (f *embeddedBrokerFakeCore) RegisterDevice(context.Context, *ingressv1.RegisterDeviceRequest) (*ingressv1.RegisterDeviceResponse, error) {
	return &ingressv1.RegisterDeviceResponse{}, nil
}

func (f *embeddedBrokerFakeCore) ReportHeartbeat(context.Context, *ingressv1.ReportHeartbeatRequest) (*ingressv1.ReportHeartbeatResponse, error) {
	return &ingressv1.ReportHeartbeatResponse{}, nil
}

func (f *embeddedBrokerFakeCore) IngestEvents(_ context.Context, req *ingressv1.IngestEventsRequest) (*ingressv1.IngestEventsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, req.GetEvents()...)
	return &ingressv1.IngestEventsResponse{}, nil
}

func (f *embeddedBrokerFakeCore) PullCommands(context.Context, *ingressv1.PullCommandsRequest) (*ingressv1.PullCommandsResponse, error) {
	return &ingressv1.PullCommandsResponse{}, nil
}

func (f *embeddedBrokerFakeCore) UpdateCommandStatus(context.Context, *ingressv1.UpdateCommandStatusRequest) (*ingressv1.UpdateCommandStatusResponse, error) {
	return &ingressv1.UpdateCommandStatusResponse{}, nil
}

func (f *embeddedBrokerFakeCore) eventCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

func (f *embeddedBrokerFakeCore) lastEvent() *ingressv1.CanonicalDeviceEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.events) == 0 {
		return nil
	}
	return f.events[len(f.events)-1]
}

func TestEmbeddedBrokerAuthenticatesWithClientIDAndPasswordToken(t *testing.T) {
	addr := freeTCPAddr(t)
	core := newEmbeddedBrokerFakeCore()
	cfg := config.Default().Adapters.MQTT
	cfg.Enabled = true
	cfg.Mode = "embedded"
	cfg.ListenAddr = addr
	cfg.DownlinkEnabled = false
	cfg.MessageBuffer = 16
	cfg.RPCTimeout = time.Second

	adapter := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)),
		WithCoreClient(core),
		WithNormalizer(normalizer.New("embedded-broker-test")),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- adapter.Start(ctx)
	}()

	client := connectEmbeddedMQTT(t, addr, "dev-1", "token-dev-1")
	defer client.Disconnect(250)

	if bad, err := tryConnectEmbeddedMQTT(addr, "dev-bad", "wrong-token"); err == nil {
		bad.Disconnect(250)
		t.Fatal("expected invalid password token to be rejected")
	}

	publishToken := client.Publish("goster/v1/dev-1/telemetry", 1, false, []byte(`{"temperature":21.5}`))
	if !publishToken.WaitTimeout(2 * time.Second) {
		t.Fatal("publish timed out")
	}
	if err := publishToken.Error(); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool { return core.eventCount() == 1 })
	event := core.lastEvent()
	if event == nil {
		t.Fatal("expected one event")
	}
	if event.GetDevice().GetUuid() != "dev-1" || event.GetContext().GetTenantId() != "tenant-a" {
		t.Fatalf("unexpected event identity: uuid=%q tenant=%q", event.GetDevice().GetUuid(), event.GetContext().GetTenantId())
	}
	if got := event.GetContext().GetLabels()["mqtt_client_id"]; got != "dev-1" {
		t.Fatalf("expected mqtt_client_id label, got %q", got)
	}

	rejected := client.Publish("goster/v1/dev-2/telemetry", 1, false, []byte(`{"temperature":99}`))
	rejected.WaitTimeout(500 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	if count := core.eventCount(); count != 1 {
		t.Fatalf("acl should reject mismatched uuid publish, event count=%d", count)
	}

	payloadTokenMismatch := client.Publish("goster/v1/dev-1/telemetry", 1, false, []byte(`{"token":"token-dev-2","temperature":88}`))
	payloadTokenMismatch.WaitTimeout(500 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	if count := core.eventCount(); count != 1 {
		t.Fatalf("payload token mismatch should be rejected, event count=%d", count)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("adapter returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("adapter did not stop")
	}
}

func connectEmbeddedMQTT(t *testing.T, addr, clientID, password string) paho.Client {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		client, err := tryConnectEmbeddedMQTT(addr, clientID, password)
		if err == nil {
			return client
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("mqtt connect failed: %v", lastErr)
	return nil
}

func tryConnectEmbeddedMQTT(addr, clientID, password string) (paho.Client, error) {
	opts := paho.NewClientOptions()
	opts.AddBroker("tcp://" + addr)
	opts.SetClientID(clientID)
	opts.SetUsername(clientID)
	opts.SetPassword(password)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(false)
	opts.SetConnectTimeout(time.Second)
	client := paho.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(2 * time.Second) {
		return client, context.DeadlineExceeded
	}
	if err := token.Error(); err != nil {
		return client, err
	}
	return client, nil
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func waitUntil(t *testing.T, timeout time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
