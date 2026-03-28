package tests

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/cookiejar"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/nhirsama/Goster-IoT/cli"
	"github.com/nhirsama/Goster-IoT/src/core"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/persistence"
	"github.com/nhirsama/Goster-IoT/src/protocol"
)

type cliRuntime struct {
	webAddr string
	tcpAddr string
	dbPath  string
	cancel  context.CancelFunc
}

func startCLIRuntime(t *testing.T) *cliRuntime {
	t.Helper()

	webAddr := reserveTCPAddr(t)
	tcpAddr := reserveTCPAddr(t)
	dbPath := filepath.Join(t.TempDir(), "cli_network_e2e.db")

	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_PATH", dbPath)
	t.Setenv("DB_DSN", "")
	t.Setenv("WEB_HTTP_ADDR", webAddr)
	t.Setenv("API_TCP_ADDR", tcpAddr)
	t.Setenv("AUTHBOSS_ROOT_URL", "http://"+webAddr)

	if err := cli.RunWithArgs(context.Background(), []string{"db", "init"}); err != nil {
		t.Fatalf("db init failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go cli.StartWithContext(ctx)

	waitForHTTPServer(t, "http://"+webAddr+"/api/v1/auth/captcha/config")
	waitForTCPServer(t, tcpAddr)

	return &cliRuntime{
		webAddr: webAddr,
		tcpAddr: tcpAddr,
		dbPath:  dbPath,
		cancel:  cancel,
	}
}

func reserveTCPAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("tcp listener is unavailable in current environment: %v", err)
	}
	defer l.Close()
	return l.Addr().String()
}

func waitForTCPServer(t *testing.T, address string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("tcp server did not start at %s", address)
}

func waitForHTTPServer(t *testing.T, url string) {
	t.Helper()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("http server did not start at %s", url)
}

func newAPIClient(t *testing.T) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to create cookie jar: %v", err)
	}
	return &http.Client{
		Jar:     jar,
		Timeout: 2 * time.Second,
	}
}

func apiJSON(t *testing.T, client *http.Client, method, url string, body []byte) (int, json.RawMessage) {
	t.Helper()

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to build request %s %s: %v", method, url, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request %s %s failed: %v", method, url, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("failed to decode response body: %v body=%s", err, string(raw))
	}
	if envelope.Code != 0 {
		t.Fatalf("unexpected api error code=%d body=%s", envelope.Code, string(raw))
	}
	return resp.StatusCode, envelope.Data
}

func registerAPIUser(t *testing.T, client *http.Client, baseURL, username, password string) {
	t.Helper()

	body := []byte(`{"username":"` + username + `","password":"` + password + `","email":"` + username + `@test.local"}`)
	status, _ := apiJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", body)
	if status != http.StatusCreated {
		t.Fatalf("unexpected register status: got %d want %d", status, http.StatusCreated)
	}
}

func enqueueDownlinkViaAPI(t *testing.T, client *http.Client, baseURL, uuid string, payload []byte) int64 {
	t.Helper()

	body := []byte(`{"command":"action_exec","payload":` + string(payload) + `}`)
	status, data := apiJSON(t, client, http.MethodPost, baseURL+"/api/v1/devices/"+uuid+"/commands", body)
	if status != http.StatusOK {
		t.Fatalf("unexpected enqueue status: got %d want %d", status, http.StatusOK)
	}

	var resp struct {
		CommandID float64 `json:"command_id"`
		Status    string  `json:"status"`
		Command   string  `json:"command"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to decode enqueue response: %v body=%s", err, string(data))
	}
	if resp.Status != string(inter.DeviceCommandStatusQueued) {
		t.Fatalf("unexpected enqueue status payload: %s", resp.Status)
	}
	if resp.Command != "action_exec" {
		t.Fatalf("unexpected enqueue command payload: %s", resp.Command)
	}
	if resp.CommandID <= 0 {
		t.Fatalf("unexpected command id: %v", resp.CommandID)
	}
	return int64(resp.CommandID)
}

func openStoreForTest(t *testing.T, dbPath string) *persistence.Store {
	t.Helper()

	store, err := persistence.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite store: %v", err)
	}
	return store
}

func startTCPSession(t *testing.T, addr string) (net.Conn, inter.ProtocolCodec, []byte) {
	t.Helper()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("failed to dial tcp gateway: %v", err)
	}

	codec := protocol.NewGosterCodec()
	privKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate client key: %v", err)
	}

	hello, err := codec.Pack(privKey.PublicKey().Bytes(), inter.CmdHandshakeInit, 0, nil, 1, false)
	if err != nil {
		t.Fatalf("failed to pack handshake: %v", err)
	}
	if _, err := conn.Write(hello); err != nil {
		t.Fatalf("failed to send handshake: %v", err)
	}

	resp, err := codec.Unpack(conn, nil)
	if err != nil {
		t.Fatalf("failed to read handshake response: %v", err)
	}
	if resp.CmdID != inter.CmdHandshakeResp || !resp.IsAck {
		t.Fatalf("unexpected handshake response: %+v", resp)
	}

	serverPub, err := ecdh.X25519().NewPublicKey(resp.Payload)
	if err != nil {
		t.Fatalf("failed to parse server public key: %v", err)
	}
	sessionKey, err := privKey.ECDH(serverPub)
	if err != nil {
		t.Fatalf("failed to derive session key: %v", err)
	}

	return conn, codec, sessionKey
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
	if packet.CmdID != wantCmd || !packet.IsAck {
		t.Fatalf("unexpected ack packet: %+v", packet)
	}
	return packet
}

func expectClosed(t *testing.T, conn net.Conn, codec inter.ProtocolCodec, sessionKey []byte) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
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
			"SELECT status FROM device_commands WHERE id = ?",
			commandID,
		).Scan(&status)
		if err == nil && status == string(want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	var status string
	if err := db.QueryRow(
		"SELECT status FROM device_commands WHERE id = ?",
		commandID,
	).Scan(&status); err != nil {
		t.Fatalf("failed to query command status: %v", err)
	}
	t.Fatalf("unexpected command status: got %s want %s", status, want)
}

func seedApprovedDevice(t *testing.T, dbPath string, meta inter.DeviceMetadata) (string, string) {
	t.Helper()

	store := openStoreForTest(t, dbPath)
	services := core.NewServices(store)
	if err := services.DeviceRegistry.RegisterDevice(meta); err != nil {
		t.Fatalf("failed to register device: %v", err)
	}
	uuid := services.DeviceRegistry.GenerateUUID(meta)
	if err := services.DeviceRegistry.ApproveDevice(uuid); err != nil {
		t.Fatalf("failed to approve device: %v", err)
	}
	stored, err := store.LoadConfig(uuid)
	if err != nil {
		t.Fatalf("failed to reload device metadata: %v", err)
	}
	return uuid, stored.Token
}

func TestCLINetworkE2ERegistrationLifecycle(t *testing.T) {
	rt := startCLIRuntime(t)

	payload := strings.Join([]string{
		"New Device",
		"SN-register",
		"00:aa:bb:cc:dd:ee",
		"hw-1",
		"sw-1",
		"cfg-1",
	}, "\x1e")

	conn, codec, sessionKey := startTCPSession(t, rt.tcpAddr)
	writePacket(t, conn, codec, []byte(payload), inter.CmdDeviceRegister, 1, sessionKey, 2)
	ack := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if len(ack.Payload) != 1 || ack.Payload[0] != byte(inter.RegistrationPending) {
		t.Fatalf("unexpected registration ack: %v", ack.Payload)
	}
	expectClosed(t, conn, codec, sessionKey)

	store := openStoreForTest(t, rt.dbPath)
	services := core.NewServices(store)
	meta := inter.DeviceMetadata{
		Name:          "New Device",
		SerialNumber:  "SN-register",
		MACAddress:    "00:aa:bb:cc:dd:ee",
		HWVersion:     "hw-1",
		SWVersion:     "sw-1",
		ConfigVersion: "cfg-1",
	}
	uuid := services.DeviceRegistry.GenerateUUID(meta)
	stored, err := store.LoadConfig(uuid)
	if err != nil {
		t.Fatalf("expected registered device metadata: %v", err)
	}
	if stored.AuthenticateStatus != inter.AuthenticatePending {
		t.Fatalf("unexpected auth status: %v", stored.AuthenticateStatus)
	}

	if err := services.DeviceRegistry.ApproveDevice(uuid); err != nil {
		t.Fatalf("failed to approve device: %v", err)
	}
	stored, err = store.LoadConfig(uuid)
	if err != nil {
		t.Fatalf("failed to reload approved device: %v", err)
	}

	conn2, codec2, sessionKey2 := startTCPSession(t, rt.tcpAddr)
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

func TestCLINetworkE2ERejectsInvalidTokenAndMalformedRegistration(t *testing.T) {
	rt := startCLIRuntime(t)

	conn, codec, sessionKey := startTCPSession(t, rt.tcpAddr)
	writePacket(t, conn, codec, []byte("invalid-token"), inter.CmdAuthVerify, 1, sessionKey, 2)
	authAck := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if len(authAck.Payload) != 1 || authAck.Payload[0] != 0x01 {
		t.Fatalf("unexpected auth failure payload: %v", authAck.Payload)
	}
	expectClosed(t, conn, codec, sessionKey)

	conn2, codec2, sessionKey2 := startTCPSession(t, rt.tcpAddr)
	defer conn2.Close()
	writePacket(t, conn2, codec2, []byte("broken\x1epayload"), inter.CmdDeviceRegister, 1, sessionKey2, 2)
	registerAck := readAck(t, conn2, codec2, sessionKey2, inter.CmdAuthAck)
	if len(registerAck.Payload) != 1 || registerAck.Payload[0] != byte(inter.RegistrationRejected) {
		t.Fatalf("unexpected malformed registration payload: %v", registerAck.Payload)
	}
	expectClosed(t, conn2, codec2, sessionKey2)
}

func TestCLINetworkE2ETelemetryAndDownlinkFlow(t *testing.T) {
	rt := startCLIRuntime(t)

	meta := inter.DeviceMetadata{
		Name:          "Telemetry Device",
		HWVersion:     "v1",
		SWVersion:     "v1",
		ConfigVersion: "v1",
		SerialNumber:  "SN-telemetry",
		MACAddress:    "00:11:22:33:44:55",
		CreatedAt:     time.Now().UTC(),
	}
	uuid, token := seedApprovedDevice(t, rt.dbPath, meta)

	store := openStoreForTest(t, rt.dbPath)
	apiClient := newAPIClient(t)
	registerAPIUser(t, apiClient, "http://"+rt.webAddr, "admin_e2e", "password123")
	commandID := enqueueDownlinkViaAPI(t, apiClient, "http://"+rt.webAddr, uuid, []byte(`{"relay":"on"}`))

	conn, codec, sessionKey := startTCPSession(t, rt.tcpAddr)
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
	writeAckPacket(t, conn, codec, nil, inter.CmdActionExec, 1, sessionKey, 3)
	time.Sleep(50 * time.Millisecond)
	waitForCommandStatus(t, rt.dbPath, commandID, inter.DeviceCommandStatusAcked)

	startTS := time.Now().Add(-2 * time.Second).UnixMilli()
	metricsPayload := buildMetricsPayload(startTS, 1000, 1, 21.5, 22.25)
	writePacket(t, conn, codec, metricsPayload, inter.CmdMetricsReport, 1, sessionKey, 4)
	readAck(t, conn, codec, sessionKey, inter.CmdMetricsReport)

	points, err := store.QueryMetrics(uuid, startTS-1, startTS+3000)
	if err != nil {
		t.Fatalf("failed to query metrics: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("unexpected metrics count: got %d want 2", len(points))
	}

	logTS := time.Now().UnixMilli()
	logPayload := buildLogPayload(logTS, inter.LogLevelWarn, "battery low")
	writePacket(t, conn, codec, logPayload, inter.CmdLogReport, 1, sessionKey, 5)
	readAck(t, conn, codec, sessionKey, inter.CmdLogReport)

	level, message := queryLogRecord(t, rt.dbPath, uuid)
	if level != "WARN" || !strings.Contains(message, "battery low") {
		t.Fatalf("unexpected warn log: level=%s message=%s", level, message)
	}

	writePacket(t, conn, codec, []byte("door opened"), inter.CmdEventReport, 1, sessionKey, 6)
	readAck(t, conn, codec, sessionKey, inter.CmdEventReport)

	level, message = queryLogRecord(t, rt.dbPath, uuid)
	if level != "EVENT" || message != "door opened" {
		t.Fatalf("unexpected event log: level=%s message=%s", level, message)
	}

	writePacket(t, conn, codec, nil, inter.CmdHeartbeat, 1, sessionKey, 7)
	readAck(t, conn, codec, sessionKey, inter.CmdHeartbeat)
}

func TestCLINetworkE2ESessionKeyRotationAndErrorReport(t *testing.T) {
	rt := startCLIRuntime(t)

	meta := inter.DeviceMetadata{
		Name:          "Rekey Device",
		HWVersion:     "v1",
		SWVersion:     "v1",
		ConfigVersion: "v1",
		SerialNumber:  "SN-rekey",
		MACAddress:    "AA:BB:CC:DD:EE:00",
		CreatedAt:     time.Now().UTC(),
	}
	uuid, token := seedApprovedDevice(t, rt.dbPath, meta)

	conn, codec, sessionKey := startTCPSession(t, rt.tcpAddr)
	defer conn.Close()

	writePacket(t, conn, codec, []byte(token), inter.CmdAuthVerify, 1, sessionKey, 2)
	authAck := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if len(authAck.Payload) != 1 || authAck.Payload[0] != 0x00 {
		t.Fatalf("unexpected auth ack payload: %v", authAck.Payload)
	}

	rekeyPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate client rekey key: %v", err)
	}
	writePacket(t, conn, codec, rekeyPriv.PublicKey().Bytes(), inter.CmdKeyExchangeUplink, 1, sessionKey, 3)
	rekeyAck := readAck(t, conn, codec, sessionKey, inter.CmdKeyExchangeDownlink)

	serverPub, err := ecdh.X25519().NewPublicKey(rekeyAck.Payload)
	if err != nil {
		t.Fatalf("failed to parse rekey server key: %v", err)
	}
	nextSessionKey, err := rekeyPriv.ECDH(serverPub)
	if err != nil {
		t.Fatalf("failed to derive next session key: %v", err)
	}

	writePacket(t, conn, codec, nil, inter.CmdHeartbeat, 1, nextSessionKey, 4)
	readAck(t, conn, codec, nextSessionKey, inter.CmdHeartbeat)

	writePacket(t, conn, codec, []byte("sensor exploded"), inter.CmdErrorReport, 1, nextSessionKey, 5)
	expectClosed(t, conn, codec, nextSessionKey)

	level, message := queryLogRecord(t, rt.dbPath, uuid)
	if level != "ERROR" || !strings.Contains(message, "sensor exploded") {
		t.Fatalf("unexpected error log record: level=%s message=%s", level, message)
	}
}
