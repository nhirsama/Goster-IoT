package customtcp

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"math"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/normalizer"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/protocol/gosterwy"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

type fakeCore struct {
	mu           sync.Mutex
	authToken    string
	authResp     *ingressv1.AuthenticateDeviceResponse
	authErr      error
	blockPull    chan struct{}
	registerResp *ingressv1.RegisterDeviceResponse
	heartbeats   []*ingressv1.ReportHeartbeatRequest
	ingested     []*ingressv1.IngestEventsRequest
	pullQueue    []*ingressv1.CanonicalCommand
	updates      []*ingressv1.UpdateCommandStatusRequest
}

func newFakeCore() *fakeCore {
	return &fakeCore{
		authResp:     &ingressv1.AuthenticateDeviceResponse{Status: ingressv1.AuthStatus_AUTH_STATUS_ACCEPTED, Uuid: "dev-1", TenantId: "tenant-a"},
		registerResp: &ingressv1.RegisterDeviceResponse{Status: ingressv1.RegistrationStatus_REGISTRATION_STATUS_ACCEPTED, Uuid: "dev-new", TenantId: "tenant-a", Credential: &ingressv1.Credential{Type: "token", Value: "new-token"}},
	}
}

func (f *fakeCore) AuthenticateDevice(ctx context.Context, req *ingressv1.AuthenticateDeviceRequest) (*ingressv1.AuthenticateDeviceResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(req.GetCredentials()) > 0 {
		f.authToken = req.GetCredentials()[0].GetValue()
	}
	if f.authErr != nil {
		return nil, f.authErr
	}
	return f.authResp, nil
}

func (f *fakeCore) RegisterDevice(ctx context.Context, req *ingressv1.RegisterDeviceRequest) (*ingressv1.RegisterDeviceResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.registerResp, nil
}

func (f *fakeCore) ReportHeartbeat(ctx context.Context, req *ingressv1.ReportHeartbeatRequest) (*ingressv1.ReportHeartbeatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.heartbeats = append(f.heartbeats, req)
	return &ingressv1.ReportHeartbeatResponse{Uuid: req.GetUuid(), TenantId: "tenant-a", Availability: ingressv1.DeviceAvailability_DEVICE_AVAILABILITY_ONLINE}, nil
}

func (f *fakeCore) IngestEvents(ctx context.Context, req *ingressv1.IngestEventsRequest) (*ingressv1.IngestEventsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ingested = append(f.ingested, req)
	return &ingressv1.IngestEventsResponse{Results: []*ingressv1.EventIngestResult{{EventId: req.GetEvents()[0].GetEventId(), Success: true}}}, nil
}

func (f *fakeCore) PullCommands(ctx context.Context, req *ingressv1.PullCommandsRequest) (*ingressv1.PullCommandsResponse, error) {
	if f.blockPull != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-f.blockPull:
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.pullQueue) == 0 {
		return &ingressv1.PullCommandsResponse{}, nil
	}
	cmd := f.pullQueue[0]
	f.pullQueue = f.pullQueue[1:]
	return &ingressv1.PullCommandsResponse{Commands: []*ingressv1.CanonicalCommand{cmd}}, nil
}

func (f *fakeCore) UpdateCommandStatus(ctx context.Context, req *ingressv1.UpdateCommandStatusRequest) (*ingressv1.UpdateCommandStatusResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, proto.Clone(req).(*ingressv1.UpdateCommandStatusRequest))
	return &ingressv1.UpdateCommandStatusResponse{Success: true, Status: req.GetStatus()}, nil
}

func (f *fakeCore) snapshot() (ingested []*ingressv1.IngestEventsRequest, updates []*ingressv1.UpdateCommandStatusRequest, heartbeats []*ingressv1.ReportHeartbeatRequest) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*ingressv1.IngestEventsRequest(nil), f.ingested...), append([]*ingressv1.UpdateCommandStatusRequest(nil), f.updates...), append([]*ingressv1.ReportHeartbeatRequest(nil), f.heartbeats...)
}

func newTestAdapter(core *fakeCore) *Adapter {
	return New(config.CustomTCPConfig{Enabled: true, ReadTimeout: time.Second, IdleTimeout: 5 * time.Minute, RPCTimeout: time.Second, RegisterAckGraceDelay: time.Millisecond, DownlinkMaxBatch: 4}, slog.New(slog.NewTextHandler(io.Discard, nil)), WithCoreClient(core), WithNormalizer(normalizer.New("ingress-test")))
}

func startPipeSession(t *testing.T, a *Adapter) (net.Conn, gosterwy.ProtocolCodec, []byte, []byte, []byte, int64) {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	go a.handleConnection(context.Background(), serverConn)
	codec := gosterwy.NewCodec()
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	clientPubKey := append([]byte(nil), priv.PublicKey().Bytes()...)
	hello, err := codec.Pack(clientPubKey, gosterwy.CmdHandshakeInit, 0, nil, 1, false)
	if err != nil {
		t.Fatalf("pack handshake: %v", err)
	}
	if _, err := clientConn.Write(hello); err != nil {
		t.Fatalf("write handshake: %v", err)
	}
	resp, err := codec.Unpack(clientConn, nil)
	if err != nil {
		t.Fatalf("read handshake: %v", err)
	}
	if resp.CmdID != gosterwy.CmdHandshakeResp || !resp.IsAck {
		t.Fatalf("unexpected handshake resp: %+v", resp)
	}
	if len(resp.Payload) != 40 {
		t.Fatalf("unexpected handshake resp payload len: %d", len(resp.Payload))
	}
	serverPubKey := append([]byte(nil), resp.Payload[:32]...)
	serverTS := int64(binary.LittleEndian.Uint64(resp.Payload[32:40]))
	serverPub, err := ecdh.X25519().NewPublicKey(serverPubKey)
	if err != nil {
		t.Fatalf("parse server pub: %v", err)
	}
	key, err := priv.ECDH(serverPub)
	if err != nil {
		t.Fatalf("derive key: %v", err)
	}
	return clientConn, codec, key, clientPubKey, serverPubKey, serverTS
}

func writeAuthPacket(t *testing.T, conn net.Conn, codec gosterwy.ProtocolCodec, token string, key []byte, clientPubKey []byte, serverPubKey []byte, serverTS int64, seq uint64) {
	t.Helper()
	writePacket(t, conn, codec, authVerifyPayload(key, clientPubKey, serverPubKey, serverTS, []byte(token)), gosterwy.CmdAuthVerify, key, seq)
}

func writePacket(t *testing.T, conn net.Conn, codec gosterwy.ProtocolCodec, payload []byte, cmd gosterwy.CmdID, key []byte, seq uint64) {
	t.Helper()
	buf, err := codec.Pack(payload, cmd, 1, key, seq, false)
	if err != nil {
		t.Fatalf("pack cmd %v: %v", cmd, err)
	}
	if _, err := conn.Write(buf); err != nil {
		t.Fatalf("write cmd %v: %v", cmd, err)
	}
}

func writeAckPacket(t *testing.T, conn net.Conn, codec gosterwy.ProtocolCodec, payload []byte, cmd gosterwy.CmdID, key []byte, seq uint64) {
	t.Helper()
	buf, err := codec.Pack(payload, cmd, 1, key, seq, true)
	if err != nil {
		t.Fatalf("pack ack cmd %v: %v", cmd, err)
	}
	if _, err := conn.Write(buf); err != nil {
		t.Fatalf("write ack cmd %v: %v", cmd, err)
	}
}

func readAck(t *testing.T, conn net.Conn, codec gosterwy.ProtocolCodec, key []byte, want gosterwy.CmdID) *gosterwy.Packet {
	t.Helper()
	pkt, err := codec.Unpack(conn, key)
	if err != nil {
		t.Fatalf("read ack %v: %v", want, err)
	}
	if pkt.CmdID != want || !pkt.IsAck {
		t.Fatalf("unexpected ack: got %+v want %v", pkt, want)
	}
	return pkt
}

func expectClosed(t *testing.T, conn net.Conn, codec gosterwy.ProtocolCodec, key []byte) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, err := codec.Unpack(conn, key)
	if err == nil {
		t.Fatal("expected connection close")
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatalf("expected close, got timeout: %v", err)
	}
}

func metricsPayload(start int64, interval uint32, dataType uint8, values ...float32) []byte {
	payload := make([]byte, 17+len(values)*4)
	binary.LittleEndian.PutUint64(payload[0:8], uint64(start))
	binary.LittleEndian.PutUint32(payload[8:12], interval)
	payload[12] = dataType
	binary.LittleEndian.PutUint32(payload[13:17], uint32(len(values)))
	for i, value := range values {
		binary.LittleEndian.PutUint32(payload[17+i*4:], math.Float32bits(value))
	}
	return payload
}

func logPayload(ts int64, level byte, msg string) []byte {
	payload := make([]byte, 11+len(msg))
	binary.LittleEndian.PutUint64(payload[0:8], uint64(ts))
	payload[8] = level
	binary.LittleEndian.PutUint16(payload[9:11], uint16(len(msg)))
	copy(payload[11:], msg)
	return payload
}

func TestHandleConnectionRejectsUnauthenticatedPackets(t *testing.T) {
	core := newFakeCore()
	a := newTestAdapter(core)
	conn, codec, key, _, _, _ := startPipeSession(t, a)
	defer conn.Close()
	writePacket(t, conn, codec, nil, gosterwy.CmdHeartbeat, key, 2)
	expectClosed(t, conn, codec, key)
}

func TestHandleConnectionAuthenticatesAndIngestsTelemetryLogEventHeartbeat(t *testing.T) {
	core := newFakeCore()
	a := newTestAdapter(core)
	conn, codec, key, clientPubKey, serverPubKey, serverTS := startPipeSession(t, a)
	defer conn.Close()

	writeAuthPacket(t, conn, codec, "token-1", key, clientPubKey, serverPubKey, serverTS, 2)
	authAck := readAck(t, conn, codec, key, gosterwy.CmdAuthAck)
	if len(authAck.Payload) != 1 || authAck.Payload[0] != 0x00 {
		t.Fatalf("unexpected auth ack: %v", authAck.Payload)
	}

	start := time.Now().Add(-time.Second).UnixMilli()
	writePacket(t, conn, codec, metricsPayload(start, 1000, 1, 21.5, 22.25), gosterwy.CmdMetricsReport, key, 3)
	readAck(t, conn, codec, key, gosterwy.CmdMetricsReport)

	writePacket(t, conn, codec, logPayload(start, 2, "battery low"), gosterwy.CmdLogReport, key, 4)
	readAck(t, conn, codec, key, gosterwy.CmdLogReport)

	writePacket(t, conn, codec, []byte("door opened"), gosterwy.CmdEventReport, key, 5)
	readAck(t, conn, codec, key, gosterwy.CmdEventReport)

	writePacket(t, conn, codec, nil, gosterwy.CmdHeartbeat, key, 6)
	readAck(t, conn, codec, key, gosterwy.CmdHeartbeat)

	ingested, _, heartbeats := core.snapshot()
	if len(ingested) != 3 {
		t.Fatalf("unexpected ingest count: %d", len(ingested))
	}
	if ingested[0].GetEvents()[0].GetEventType() != ingressv1.EventType_EVENT_TYPE_TELEMETRY || len(ingested[0].GetEvents()[0].GetMetrics()) != 2 {
		t.Fatalf("unexpected metrics event: %+v", ingested[0].GetEvents()[0])
	}
	if ingested[1].GetEvents()[0].GetLogs()[0].GetMessage() != "battery low" {
		t.Fatalf("unexpected log event: %+v", ingested[1].GetEvents()[0])
	}
	if string(ingested[2].GetEvents()[0].GetRaw().GetBody()) != "door opened" {
		t.Fatalf("unexpected raw event: %+v", ingested[2].GetEvents()[0])
	}
	if len(heartbeats) != 1 || heartbeats[0].GetUuid() != "dev-1" {
		t.Fatalf("unexpected heartbeats: %+v", heartbeats)
	}
}

func TestHandleConnectionRegistrationLifecycle(t *testing.T) {
	core := newFakeCore()
	a := newTestAdapter(core)
	conn, codec, key, _, _, _ := startPipeSession(t, a)
	defer conn.Close()

	payload := strings.Join([]string{"New Device", "SN1", "AA:BB", "hw", "sw", "cfg"}, "\x1e")
	writePacket(t, conn, codec, []byte(payload), gosterwy.CmdDeviceRegister, key, 2)
	ack := readAck(t, conn, codec, key, gosterwy.CmdAuthAck)
	if len(ack.Payload) != 1+len("new-token") || ack.Payload[0] != 0x00 || string(ack.Payload[1:]) != "new-token" {
		t.Fatalf("unexpected registration ack: %v", ack.Payload)
	}
}

func TestHandleConnectionRejectsMalformedRegistration(t *testing.T) {
	core := newFakeCore()
	a := newTestAdapter(core)
	conn, codec, key, _, _, _ := startPipeSession(t, a)
	defer conn.Close()

	writePacket(t, conn, codec, []byte("bad"), gosterwy.CmdDeviceRegister, key, 2)
	ack := readAck(t, conn, codec, key, gosterwy.CmdAuthAck)
	if len(ack.Payload) != 1 || ack.Payload[0] != 0x01 {
		t.Fatalf("unexpected malformed registration ack: %v", ack.Payload)
	}
	expectClosed(t, conn, codec, key)
}

func TestHandleConnectionDownlinkSentAckedAndRequeuedOnDisconnect(t *testing.T) {
	core := newFakeCore()
	core.pullQueue = []*ingressv1.CanonicalCommand{{
		CommandId: 42,
		Uuid:      "dev-1",
		Operation: "config_push",
		Payload:   &ingressv1.RawPayload{ContentType: "application/json", Body: []byte(`{"sampling":"5s"}`)},
	}}
	a := newTestAdapter(core)
	conn, codec, key, clientPubKey, serverPubKey, serverTS := startPipeSession(t, a)

	writeAuthPacket(t, conn, codec, "token-1", key, clientPubKey, serverPubKey, serverTS, 2)
	readAck(t, conn, codec, key, gosterwy.CmdAuthAck)

	downlink, err := codec.Unpack(conn, key)
	if err != nil {
		t.Fatalf("read downlink: %v", err)
	}
	if downlink.CmdID != gosterwy.CmdConfigPush || downlink.IsAck || !bytes.Equal(downlink.Payload, []byte(`{"sampling":"5s"}`)) {
		t.Fatalf("unexpected downlink: %+v", downlink)
	}
	var updates []*ingressv1.UpdateCommandStatusRequest
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, updates, _ = core.snapshot()
		if len(updates) >= 1 && updates[0].GetStatus() == ingressv1.CommandStatus_COMMAND_STATUS_SENT {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(updates) < 1 || updates[0].GetStatus() != ingressv1.CommandStatus_COMMAND_STATUS_SENT {
		t.Fatalf("expected sent update, got %+v", updates)
	}

	writeAckPacket(t, conn, codec, nil, gosterwy.CmdConfigPush, key, 3)
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, updates, _ = core.snapshot()
		if len(updates) >= 2 && updates[1].GetStatus() == ingressv1.CommandStatus_COMMAND_STATUS_ACKED {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, updates, _ = core.snapshot()
	if len(updates) < 2 || updates[1].GetStatus() != ingressv1.CommandStatus_COMMAND_STATUS_ACKED {
		t.Fatalf("expected ack update, got %+v", updates)
	}
	_ = conn.Close()

	core = newFakeCore()
	core.pullQueue = []*ingressv1.CanonicalCommand{{CommandId: 43, Uuid: "dev-1", Operation: "action_exec", Payload: &ingressv1.RawPayload{Body: []byte("on")}}}
	a = newTestAdapter(core)
	conn, codec, key, clientPubKey, serverPubKey, serverTS = startPipeSession(t, a)
	writeAuthPacket(t, conn, codec, "token-1", key, clientPubKey, serverPubKey, serverTS, 2)
	readAck(t, conn, codec, key, gosterwy.CmdAuthAck)
	if _, err := codec.Unpack(conn, key); err != nil {
		t.Fatalf("read second downlink: %v", err)
	}
	_ = conn.Close()
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, updates, _ = core.snapshot()
		if len(updates) >= 2 && updates[len(updates)-1].GetStatus() == ingressv1.CommandStatus_COMMAND_STATUS_REQUEUED {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, updates, _ = core.snapshot()
	t.Fatalf("expected requeued update after disconnect, got %+v", updates)
}

func TestHandleConnectionRejectsStaleDownlinkAckSequence(t *testing.T) {
	core := newFakeCore()
	core.pullQueue = []*ingressv1.CanonicalCommand{{
		CommandId: 42,
		Uuid:      "dev-1",
		Operation: "config_push",
		Payload:   &ingressv1.RawPayload{Body: []byte(`{"sampling":"5s"}`)},
	}}
	a := newTestAdapter(core)
	conn, codec, key, clientPubKey, serverPubKey, serverTS := startPipeSession(t, a)
	defer conn.Close()

	writeAuthPacket(t, conn, codec, "token-1", key, clientPubKey, serverPubKey, serverTS, 2)
	readAck(t, conn, codec, key, gosterwy.CmdAuthAck)
	downlink, err := codec.Unpack(conn, key)
	if err != nil {
		t.Fatalf("read downlink: %v", err)
	}

	writeAckPacket(t, conn, codec, nil, gosterwy.CmdConfigPush, key, downlink.Sequence-1)
	time.Sleep(50 * time.Millisecond)
	_, updates, _ := core.snapshot()
	for _, update := range updates {
		if update.GetStatus() == ingressv1.CommandStatus_COMMAND_STATUS_ACKED {
			t.Fatalf("stale ack must not mark command acked: %+v", updates)
		}
	}

	writeAckPacket(t, conn, codec, nil, gosterwy.CmdConfigPush, key, downlink.Sequence)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, updates, _ = core.snapshot()
		for _, update := range updates {
			if update.GetStatus() == ingressv1.CommandStatus_COMMAND_STATUS_ACKED {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected matching ack to update command, got %+v", updates)
}

func TestHandleConnectionRejectsAuthWithoutHandshakeHMAC(t *testing.T) {
	core := newFakeCore()
	a := newTestAdapter(core)
	conn, codec, key, _, _, _ := startPipeSession(t, a)
	defer conn.Close()

	writePacket(t, conn, codec, []byte("token-1"), gosterwy.CmdAuthVerify, key, 2)
	authAck := readAck(t, conn, codec, key, gosterwy.CmdAuthAck)
	if len(authAck.Payload) != 1 || authAck.Payload[0] != 0x01 {
		t.Fatalf("unexpected auth ack for unsigned payload: %v", authAck.Payload)
	}
	expectClosed(t, conn, codec, key)
}

func TestHandleConnectionIdleTimeoutClosesBlockedConnection(t *testing.T) {
	core := newFakeCore()
	core.blockPull = make(chan struct{})
	a := New(config.CustomTCPConfig{Enabled: true, ReadTimeout: time.Second, IdleTimeout: 50 * time.Millisecond, RPCTimeout: 500 * time.Millisecond, RegisterAckGraceDelay: time.Millisecond, DownlinkMaxBatch: 1}, slog.New(slog.NewTextHandler(io.Discard, nil)), WithCoreClient(core), WithNormalizer(normalizer.New("ingress-test")))
	conn, codec, key, clientPubKey, serverPubKey, serverTS := startPipeSession(t, a)
	defer conn.Close()

	writeAuthPacket(t, conn, codec, "token-1", key, clientPubKey, serverPubKey, serverTS, 2)
	readAck(t, conn, codec, key, gosterwy.CmdAuthAck)
	expectClosed(t, conn, codec, key)
}

func TestHandleConnectionReportsDeviceErrorAndCloses(t *testing.T) {
	core := newFakeCore()
	a := newTestAdapter(core)
	conn, codec, key, clientPubKey, serverPubKey, serverTS := startPipeSession(t, a)
	defer conn.Close()
	writeAuthPacket(t, conn, codec, "token-1", key, clientPubKey, serverPubKey, serverTS, 2)
	readAck(t, conn, codec, key, gosterwy.CmdAuthAck)
	writePacket(t, conn, codec, []byte("sensor exploded"), gosterwy.CmdErrorReport, key, 3)
	expectClosed(t, conn, codec, key)
	ingested, _, _ := core.snapshot()
	if len(ingested) != 1 || ingested[0].GetEvents()[0].GetEventType() != ingressv1.EventType_EVENT_TYPE_DEVICE_ERROR || ingested[0].GetEvents()[0].GetLogs()[0].GetMessage() != "sensor exploded" {
		t.Fatalf("unexpected error ingest: %+v", ingested)
	}
}

func TestHandleConnectionSupportsSessionKeyRotation(t *testing.T) {
	core := newFakeCore()
	a := newTestAdapter(core)
	conn, codec, key, clientPubKey, serverPubKey, serverTS := startPipeSession(t, a)
	defer conn.Close()
	writeAuthPacket(t, conn, codec, "token-1", key, clientPubKey, serverPubKey, serverTS, 2)
	readAck(t, conn, codec, key, gosterwy.CmdAuthAck)

	rekeyPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	writePacket(t, conn, codec, rekeyPriv.PublicKey().Bytes(), gosterwy.CmdKeyExchangeUplink, key, 3)
	rekeyResp := readAck(t, conn, codec, key, gosterwy.CmdKeyExchangeDownlink)
	serverPub, err := ecdh.X25519().NewPublicKey(rekeyResp.Payload)
	if err != nil {
		t.Fatalf("parse rekey pub: %v", err)
	}
	nextKey, err := rekeyPriv.ECDH(serverPub)
	if err != nil {
		t.Fatalf("derive rekey: %v", err)
	}
	writePacket(t, conn, codec, nil, gosterwy.CmdHeartbeat, nextKey, 4)
	readAck(t, conn, codec, nextKey, gosterwy.CmdHeartbeat)
}

func TestResolveDownlinkCmdFromOptions(t *testing.T) {
	opts, err := structpb.NewStruct(map[string]any{"command_code": float64(0x0203)})
	if err != nil {
		t.Fatalf("NewStruct: %v", err)
	}
	cmd, err := normalizer.New("x").NormalizeCommand(context.Background(), &ingressv1.CanonicalCommand{CommandId: 1, Operation: "unknown", AdapterOptions: opts})
	if err != nil {
		t.Fatalf("NormalizeCommand: %v", err)
	}
	got, ok := resolveDownlinkCmd(cmd)
	if !ok || got != gosterwy.CmdActionExec {
		t.Fatalf("unexpected cmd mapping: %v %v", got, ok)
	}
}
