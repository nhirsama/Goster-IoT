package api

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
	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/device_manager"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
	"github.com/nhirsama/Goster-IoT/src/protocol"
)

type apiTestHarness struct {
	api    *apiImpl
	ds     inter.DataStore
	dm     inter.DeviceManager
	dbPath string
}

func newAPITestHarness(t *testing.T) *apiTestHarness {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "api_test.db")
	ds, err := datastore.NewDataStoreSql(dbPath)
	if err != nil {
		t.Fatalf("failed to init datastore: %v", err)
	}
	dm := device_manager.NewDeviceManager(ds)
	apiSvc := NewApiWithConfig(ds, dm, logger.NewNoop(), appcfg.APIConfig{
		ReadTimeout:           5 * time.Second,
		RegisterAckGraceDelay: 5 * time.Millisecond,
	})
	impl, ok := apiSvc.(*apiImpl)
	if !ok {
		t.Fatalf("unexpected api implementation type: %T", apiSvc)
	}

	return &apiTestHarness{
		api:    impl,
		ds:     ds,
		dm:     dm,
		dbPath: dbPath,
	}
}

func startPipeSession(t *testing.T, api *apiImpl) (net.Conn, inter.ProtocolCodec, []byte) {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	go api.handleConnection(serverConn)

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

func seedApprovedDevice(t *testing.T, h *apiTestHarness, meta inter.DeviceMetadata) (string, string) {
	t.Helper()

	if err := h.dm.RegisterDevice(meta); err != nil {
		t.Fatalf("failed to register device: %v", err)
	}
	uuid := h.dm.GenerateUUID(meta)
	if err := h.dm.ApproveDevice(uuid); err != nil {
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

func TestHandleConnectionAuthenticateAndPersistTelemetry(t *testing.T) {
	h := newAPITestHarness(t)
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

	conn, codec, sessionKey := startPipeSession(t, h.api)
	defer conn.Close()

	writePacket(t, conn, codec, []byte(token), inter.CmdAuthVerify, 1, sessionKey, 2)
	authAck := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if got := authAck.Payload[0]; got != 0x00 {
		t.Fatalf("auth should succeed, got status %d", got)
	}

	startTS := time.Now().UnixMilli()
	metrics := buildMetricsPayload(startTS, 1000, 1, 23.5, 24.25)
	writePacket(t, conn, codec, metrics, inter.CmdMetricsReport, 1, sessionKey, 3)
	readAck(t, conn, codec, sessionKey, inter.CmdMetricsReport)

	logPayload := buildLogPayload(time.Now().UnixMilli(), inter.LogLevelWarn, "fan speed degraded")
	writePacket(t, conn, codec, logPayload, inter.CmdLogReport, 1, sessionKey, 4)
	readAck(t, conn, codec, sessionKey, inter.CmdLogReport)

	points, err := h.ds.QueryMetrics(uuid, startTS-1000, startTS+5000)
	if err != nil {
		t.Fatalf("failed to query metrics: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("unexpected metric count: got %d want 2", len(points))
	}
	if math.Abs(float64(points[0].Value-23.5)) > 0.001 {
		t.Fatalf("unexpected first metric value: got %f want 23.5", points[0].Value)
	}
	if points[1].Timestamp != startTS+1000 {
		t.Fatalf("unexpected second metric timestamp: got %d want %d", points[1].Timestamp, startTS+1000)
	}

	level, message := queryLogRecord(t, h.dbPath, uuid)
	if level != "WARN" {
		t.Fatalf("unexpected log level: got %s want WARN", level)
	}
	if message == "" || !strings.Contains(message, "fan speed degraded") {
		t.Fatalf("unexpected log message: %q", message)
	}
}

func TestHandleConnectionRejectsInvalidToken(t *testing.T) {
	h := newAPITestHarness(t)
	conn, codec, sessionKey := startPipeSession(t, h.api)
	defer conn.Close()

	writePacket(t, conn, codec, []byte("bad_token"), inter.CmdAuthVerify, 1, sessionKey, 2)
	authAck := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if got := authAck.Payload[0]; got != 0x01 {
		t.Fatalf("auth should fail, got status %d", got)
	}

	expectClosed(t, conn, codec, sessionKey)
}

func TestHandleConnectionRejectsUnauthenticatedMetrics(t *testing.T) {
	h := newAPITestHarness(t)
	conn, codec, sessionKey := startPipeSession(t, h.api)
	defer conn.Close()

	writePacket(t, conn, codec, buildMetricsPayload(time.Now().UnixMilli(), 1000, 1, 20.5), inter.CmdMetricsReport, 1, sessionKey, 2)
	expectClosed(t, conn, codec, sessionKey)
}

func TestHandleConnectionRegistrationFlow(t *testing.T) {
	h := newAPITestHarness(t)
	meta := inter.DeviceMetadata{
		Name:          "Register Device",
		HWVersion:     "v2",
		SWVersion:     "v3",
		ConfigVersion: "cfg-1",
		SerialNumber:  "SN-register",
		MACAddress:    "AA:BB:CC:DD:EE:FF",
	}
	regPayload := []byte(strings.Join([]string{
		meta.Name,
		meta.SerialNumber,
		meta.MACAddress,
		meta.HWVersion,
		meta.SWVersion,
		meta.ConfigVersion,
	}, "\x1e"))

	conn, codec, sessionKey := startPipeSession(t, h.api)
	writePacket(t, conn, codec, regPayload, inter.CmdDeviceRegister, 1, sessionKey, 2)
	pendingAck := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if got := pendingAck.Payload[0]; got != 0x02 {
		t.Fatalf("first registration should be pending, got %d", got)
	}
	expectClosed(t, conn, codec, sessionKey)

	uuid := h.dm.GenerateUUID(meta)
	stored, err := h.ds.LoadConfig(uuid)
	if err != nil {
		t.Fatalf("failed to load registered device: %v", err)
	}
	if stored.AuthenticateStatus != inter.AuthenticatePending {
		t.Fatalf("unexpected auth status after first registration: got %v want %v", stored.AuthenticateStatus, inter.AuthenticatePending)
	}

	if err := h.dm.ApproveDevice(uuid); err != nil {
		t.Fatalf("failed to approve device: %v", err)
	}
	stored, err = h.ds.LoadConfig(uuid)
	if err != nil {
		t.Fatalf("failed to reload approved device: %v", err)
	}
	if stored.Token == "" {
		t.Fatal("approved device should have a token")
	}

	conn2, codec2, sessionKey2 := startPipeSession(t, h.api)
	defer conn2.Close()
	writePacket(t, conn2, codec2, regPayload, inter.CmdDeviceRegister, 1, sessionKey2, 2)
	successAck := readAck(t, conn2, codec2, sessionKey2, inter.CmdAuthAck)
	if got := successAck.Payload[0]; got != 0x00 {
		t.Fatalf("approved registration should succeed, got %d", got)
	}
	if token := string(successAck.Payload[1:]); token != stored.Token {
		t.Fatalf("unexpected returned token: got %q want %q", token, stored.Token)
	}
}
