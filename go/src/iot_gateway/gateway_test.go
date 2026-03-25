package iot_gateway

import (
	"crypto/ecdh"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/core"
	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
	"github.com/nhirsama/Goster-IoT/src/protocol"
)

type gatewayTestHarness struct {
	gateway          *gatewayService
	ds               inter.DataStore
	registry         inter.DeviceRegistry
	downlinkCommands inter.DownlinkCommandService
	dbPath           string
}

func newGatewayTestHarness(t *testing.T) *gatewayTestHarness {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "gateway_test.db")
	ds, err := datastore.NewDataStoreSql(dbPath)
	if err != nil {
		t.Fatalf("failed to init datastore: %v", err)
	}
	services := core.NewServices(ds)
	svc := NewGatewayFromCoreWithConfig(services.DeviceRegistry, services.DevicePresence, services.TelemetryIngest, services.DownlinkCommands, logger.NewNoop(), appcfg.APIConfig{
		ReadTimeout:           5 * time.Second,
		RegisterAckGraceDelay: 5 * time.Millisecond,
	})
	impl, ok := svc.(*gatewayService)
	if !ok {
		t.Fatalf("unexpected gateway implementation type: %T", svc)
	}

	return &gatewayTestHarness{
		gateway:          impl,
		ds:               ds,
		registry:         services.DeviceRegistry,
		downlinkCommands: services.DownlinkCommands,
		dbPath:           dbPath,
	}
}

func startPipeSession(t *testing.T, gateway *gatewayService) (net.Conn, inter.ProtocolCodec, []byte) {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	go gateway.handleConnection(serverConn)

	codec := protocol.NewGosterCodec()
	privKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate client key: %v", err)
	}

	hello, err := codec.Pack(privKey.PublicKey().Bytes(), inter.CmdHandshakeInit, 0, nil, 1, false)
	if err != nil {
		t.Fatalf("failed to pack handshake: %v", err)
	}
	if _, err := clientConn.Write(hello); err != nil {
		t.Fatalf("failed to send handshake: %v", err)
	}

	resp, err := codec.Unpack(clientConn, nil)
	if err != nil {
		t.Fatalf("failed to read handshake response: %v", err)
	}
	if resp.CmdID != inter.CmdHandshakeResp {
		t.Fatalf("unexpected handshake response cmd: got %v want %v", resp.CmdID, inter.CmdHandshakeResp)
	}
	if !resp.IsAck {
		t.Fatal("handshake response should be marked as ack")
	}

	serverPub, err := ecdh.X25519().NewPublicKey(resp.Payload)
	if err != nil {
		t.Fatalf("failed to parse server public key: %v", err)
	}
	sessionKey, err := privKey.ECDH(serverPub)
	if err != nil {
		t.Fatalf("failed to derive session key: %v", err)
	}

	return clientConn, codec, sessionKey
}

func seedApprovedDevice(t *testing.T, h *gatewayTestHarness, meta inter.DeviceMetadata) (string, string) {
	t.Helper()

	if err := h.registry.RegisterDevice(meta); err != nil {
		t.Fatalf("failed to register device: %v", err)
	}
	uuid := h.registry.GenerateUUID(meta)
	if err := h.registry.ApproveDevice(uuid); err != nil {
		t.Fatalf("failed to approve device: %v", err)
	}
	stored, err := h.ds.LoadConfig(uuid)
	if err != nil {
		t.Fatalf("failed to reload device metadata: %v", err)
	}
	if stored.Token == "" {
		t.Fatal("approved device should have a token")
	}

	return uuid, stored.Token
}

func writePacket(t *testing.T, conn net.Conn, codec inter.ProtocolCodec, payload []byte, cmd inter.CmdID, keyID uint32, sessionKey []byte, seq uint64) {
	t.Helper()

	buf, err := codec.Pack(payload, cmd, keyID, sessionKey, seq, false)
	if err != nil {
		t.Fatalf("failed to pack packet cmd=%v: %v", cmd, err)
	}
	if _, err := conn.Write(buf); err != nil {
		t.Fatalf("failed to write packet cmd=%v: %v", cmd, err)
	}
}

func writeAckPacket(t *testing.T, conn net.Conn, codec inter.ProtocolCodec, payload []byte, cmd inter.CmdID, keyID uint32, sessionKey []byte, seq uint64) {
	t.Helper()

	buf, err := codec.Pack(payload, cmd, keyID, sessionKey, seq, true)
	if err != nil {
		t.Fatalf("failed to pack ack packet cmd=%v: %v", cmd, err)
	}
	if _, err := conn.Write(buf); err != nil {
		t.Fatalf("failed to write ack packet cmd=%v: %v", cmd, err)
	}
}

func readAck(t *testing.T, conn net.Conn, codec inter.ProtocolCodec, sessionKey []byte, wantCmd inter.CmdID) *inter.Packet {
	t.Helper()

	packet, err := codec.Unpack(conn, sessionKey)
	if err != nil {
		t.Fatalf("failed to read ack cmd=%v: %v", wantCmd, err)
	}
	if packet.CmdID != wantCmd {
		t.Fatalf("unexpected ack cmd: got %v want %v", packet.CmdID, wantCmd)
	}
	if !packet.IsAck {
		t.Fatalf("packet cmd=%v should be ack", packet.CmdID)
	}
	return packet
}

func expectClosed(t *testing.T, conn net.Conn, codec inter.ProtocolCodec, sessionKey []byte) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "closed pipe") {
			return
		}
		t.Fatalf("failed to set read deadline: %v", err)
	}
	_, err := codec.Unpack(conn, sessionKey)
	if err == nil {
		t.Fatal("expected connection to close")
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatalf("expected connection close, got timeout: %v", err)
	}
	if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF/closed error, got %v", err)
	}
}

func buildMetricsPayload(start int64, interval uint32, dataType uint8, values ...float32) []byte {
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

func buildLogPayload(ts int64, level inter.LogLevel, msg string) []byte {
	payload := make([]byte, 11+len(msg))
	binary.LittleEndian.PutUint64(payload[0:8], uint64(ts))
	payload[8] = byte(level)
	binary.LittleEndian.PutUint16(payload[9:11], uint16(len(msg)))
	copy(payload[11:], []byte(msg))
	return payload
}

func queryLogRecord(t *testing.T, dbPath, uuid string) (string, string) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	var level string
	var message string
	if err := db.QueryRow("SELECT level, message FROM logs WHERE uuid = ? ORDER BY id DESC LIMIT 1", uuid).Scan(&level, &message); err != nil {
		t.Fatalf("failed to query log record: %v", err)
	}

	return level, message
}

func waitForCommandStatus(t *testing.T, dbPath string, commandID int64, want inter.DeviceCommandStatus) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		var status string
		err := db.QueryRow(
			"SELECT status FROM integration_external_commands WHERE id = ? AND source = ?",
			commandID,
			"goster_device",
		).Scan(&status)
		if err == nil && status == string(want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	var status string
	if err := db.QueryRow(
		"SELECT status FROM integration_external_commands WHERE id = ? AND source = ?",
		commandID,
		"goster_device",
	).Scan(&status); err != nil {
		t.Fatalf("failed to query command status: %v", err)
	}
	t.Fatalf("unexpected command status: got %s want %s", status, want)
}

func TestHandleConnectionAuthenticateAndPersistTelemetry(t *testing.T) {
	h := newGatewayTestHarness(t)
	meta := inter.DeviceMetadata{
		Name:          "Telemetry Device",
		HWVersion:     "v1",
		SWVersion:     "v1",
		ConfigVersion: "v1",
		SerialNumber:  "SN-telemetry",
		MACAddress:    "00:11:22:33:44:55",
		CreatedAt:     time.Now().UTC(),
	}
	uuid, token := seedApprovedDevice(t, h, meta)

	conn, codec, sessionKey := startPipeSession(t, h.gateway)
	defer conn.Close()

	writePacket(t, conn, codec, []byte(token), inter.CmdAuthVerify, 1, sessionKey, 2)
	authAck := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if len(authAck.Payload) != 1 || authAck.Payload[0] != 0x00 {
		t.Fatalf("unexpected auth ack payload: %v", authAck.Payload)
	}

	startTS := time.Now().Add(-2 * time.Second).UnixMilli()
	metricsPayload := buildMetricsPayload(startTS, 1000, 1, 21.5, 22.25)
	writePacket(t, conn, codec, metricsPayload, inter.CmdMetricsReport, 1, sessionKey, 3)
	readAck(t, conn, codec, sessionKey, inter.CmdMetricsReport)

	points, err := h.ds.QueryMetrics(uuid, startTS-1, startTS+3000)
	if err != nil {
		t.Fatalf("failed to query metrics: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("unexpected metrics count: got %d want 2", len(points))
	}
	if points[0].Timestamp != startTS || points[1].Timestamp != startTS+1000 {
		t.Fatalf("unexpected metric timestamps: %+v", points)
	}
	if points[0].Value != float32(21.5) || points[1].Value != float32(22.25) {
		t.Fatalf("unexpected metric values: %+v", points)
	}

	logTS := time.Now().UnixMilli()
	logPayload := buildLogPayload(logTS, inter.LogLevelWarn, "battery low")
	writePacket(t, conn, codec, logPayload, inter.CmdLogReport, 1, sessionKey, 4)
	readAck(t, conn, codec, sessionKey, inter.CmdLogReport)

	level, message := queryLogRecord(t, h.dbPath, uuid)
	if level != "WARN" {
		t.Fatalf("unexpected log level: got %s want WARN", level)
	}
	if !strings.Contains(message, "battery low") {
		t.Fatalf("unexpected log message: %s", message)
	}

	writePacket(t, conn, codec, []byte("door opened"), inter.CmdEventReport, 1, sessionKey, 5)
	readAck(t, conn, codec, sessionKey, inter.CmdEventReport)

	level, message = queryLogRecord(t, h.dbPath, uuid)
	if level != "EVENT" || message != "door opened" {
		t.Fatalf("unexpected event log record: level=%s message=%s", level, message)
	}
}

func TestHandleConnectionRejectsUnauthenticatedPackets(t *testing.T) {
	h := newGatewayTestHarness(t)
	conn, codec, sessionKey := startPipeSession(t, h.gateway)
	defer conn.Close()

	writePacket(t, conn, codec, nil, inter.CmdHeartbeat, 1, sessionKey, 2)
	expectClosed(t, conn, codec, sessionKey)
}

func TestHandleConnectionSupportsRegistrationLifecycle(t *testing.T) {
	h := newGatewayTestHarness(t)

	payload := strings.Join([]string{
		"New Device",
		"SN-register",
		"00:aa:bb:cc:dd:ee",
		"hw-1",
		"sw-1",
		"cfg-1",
	}, "\x1e")

	conn, codec, sessionKey := startPipeSession(t, h.gateway)
	writePacket(t, conn, codec, []byte(payload), inter.CmdDeviceRegister, 1, sessionKey, 2)
	ack := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if len(ack.Payload) != 1 || ack.Payload[0] != byte(inter.RegistrationPending) {
		t.Fatalf("unexpected registration ack: %v", ack.Payload)
	}
	expectClosed(t, conn, codec, sessionKey)

	meta := inter.DeviceMetadata{
		Name:          "New Device",
		SerialNumber:  "SN-register",
		MACAddress:    "00:aa:bb:cc:dd:ee",
		HWVersion:     "hw-1",
		SWVersion:     "sw-1",
		ConfigVersion: "cfg-1",
	}
	uuid := h.registry.GenerateUUID(meta)

	stored, err := h.ds.LoadConfig(uuid)
	if err != nil {
		t.Fatalf("expected registered device metadata: %v", err)
	}
	if stored.AuthenticateStatus != inter.AuthenticatePending {
		t.Fatalf("unexpected device status: got %v want pending", stored.AuthenticateStatus)
	}

	if err := h.registry.ApproveDevice(uuid); err != nil {
		t.Fatalf("failed to approve device: %v", err)
	}
	stored, err = h.ds.LoadConfig(uuid)
	if err != nil {
		t.Fatalf("failed to reload approved device: %v", err)
	}

	conn2, codec2, sessionKey2 := startPipeSession(t, h.gateway)
	defer conn2.Close()
	writePacket(t, conn2, codec2, []byte(payload), inter.CmdDeviceRegister, 1, sessionKey2, 2)
	ack = readAck(t, conn2, codec2, sessionKey2, inter.CmdAuthAck)
	if len(ack.Payload) < 2 || ack.Payload[0] != byte(inter.RegistrationAccepted) {
		t.Fatalf("unexpected approved registration ack: %v", ack.Payload)
	}
	if gotToken := string(ack.Payload[1:]); gotToken != stored.Token {
		t.Fatalf("unexpected token from registration ack: got %s want %s", gotToken, stored.Token)
	}
}

func TestHandleConnectionMarksDownlinkAcked(t *testing.T) {
	h := newGatewayTestHarness(t)
	meta := inter.DeviceMetadata{
		Name:          "Actuator",
		HWVersion:     "v1",
		SWVersion:     "v1",
		ConfigVersion: "v1",
		SerialNumber:  "SN-downlink",
		MACAddress:    "66:55:44:33:22:11",
		CreatedAt:     time.Now().UTC(),
	}
	uuid, token := seedApprovedDevice(t, h, meta)

	msg, err := h.downlinkCommands.Enqueue(inter.Scope{}, uuid, inter.CmdActionExec, "toggle", []byte(`{"state":"on"}`))
	if err != nil {
		t.Fatalf("failed to enqueue downlink command: %v", err)
	}
	commandID := msg.CommandID

	conn, codec, sessionKey := startPipeSession(t, h.gateway)
	defer conn.Close()

	writePacket(t, conn, codec, []byte(token), inter.CmdAuthVerify, 1, sessionKey, 2)
	authAck := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if len(authAck.Payload) != 1 || authAck.Payload[0] != 0x00 {
		t.Fatalf("unexpected auth ack payload: %v", authAck.Payload)
	}

	downlink, err := codec.Unpack(conn, sessionKey)
	if err != nil {
		t.Fatalf("failed to read downlink packet: %v", err)
	}
	if downlink.CmdID != inter.CmdActionExec || downlink.IsAck {
		t.Fatalf("unexpected downlink packet: %+v", downlink)
	}

	time.Sleep(50 * time.Millisecond)
	waitForCommandStatus(t, h.dbPath, commandID, inter.DeviceCommandStatusSent)
	writeAckPacket(t, conn, codec, nil, inter.CmdActionExec, 1, sessionKey, 3)

	time.Sleep(50 * time.Millisecond)
	waitForCommandStatus(t, h.dbPath, commandID, inter.DeviceCommandStatusAcked)
}
