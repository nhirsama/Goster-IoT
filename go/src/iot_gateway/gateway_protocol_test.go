package iot_gateway

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"database/sql"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/protocol"
)

func TestHandleConnectionRejectsMalformedHandshake(t *testing.T) {
	h := newGatewayTestHarness(t)

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go h.gateway.handleConnection(serverConn)

	codec := protocol.NewGosterCodec()
	buf, err := codec.Pack([]byte("short"), inter.CmdHandshakeInit, 0, nil, 1, false)
	if err != nil {
		t.Fatalf("failed to pack malformed handshake: %v", err)
	}
	if _, err := clientConn.Write(buf); err != nil {
		t.Fatalf("failed to write malformed handshake: %v", err)
	}

	expectClosed(t, clientConn, codec, nil)
}

func TestHandleConnectionRejectsInvalidToken(t *testing.T) {
	h := newGatewayTestHarness(t)

	conn, codec, sessionKey := startPipeSession(t, h.gateway)
	defer conn.Close()

	writePacket(t, conn, codec, []byte("invalid-token"), inter.CmdAuthVerify, 1, sessionKey, 2)
	authAck := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if len(authAck.Payload) != 1 || authAck.Payload[0] != 0x01 {
		t.Fatalf("unexpected auth failure ack payload: %v", authAck.Payload)
	}

	expectClosed(t, conn, codec, sessionKey)
}

func TestHandleConnectionRejectsMalformedRegistrationPayload(t *testing.T) {
	h := newGatewayTestHarness(t)

	conn, codec, sessionKey := startPipeSession(t, h.gateway)
	defer conn.Close()

	writePacket(t, conn, codec, []byte("only\x1ethree\x1efields"), inter.CmdDeviceRegister, 1, sessionKey, 2)
	ack := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if len(ack.Payload) != 1 || ack.Payload[0] != byte(inter.RegistrationRejected) {
		t.Fatalf("unexpected malformed registration ack: %v", ack.Payload)
	}

	items, err := h.ds.ListDevices(1, 10)
	if err != nil {
		t.Fatalf("failed to list devices: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("malformed registration should not create device records: %+v", items)
	}

	expectClosed(t, conn, codec, sessionKey)
}

func TestHandleConnectionRejectsRefusedRegistration(t *testing.T) {
	h := newGatewayTestHarness(t)

	meta := inter.DeviceMetadata{
		Name:          "Refused Device",
		SerialNumber:  "SN-refused",
		MACAddress:    "10:20:30:40:50:60",
		HWVersion:     "hw-1",
		SWVersion:     "sw-1",
		ConfigVersion: "cfg-1",
	}
	if err := h.registry.RegisterDevice(meta); err != nil {
		t.Fatalf("failed to register device: %v", err)
	}
	uuid := h.registry.GenerateUUID(meta)
	if err := h.registry.RejectDevice(uuid); err != nil {
		t.Fatalf("failed to reject device: %v", err)
	}

	payload := strings.Join([]string{
		meta.Name,
		meta.SerialNumber,
		meta.MACAddress,
		meta.HWVersion,
		meta.SWVersion,
		meta.ConfigVersion,
	}, "\x1e")

	conn, codec, sessionKey := startPipeSession(t, h.gateway)
	defer conn.Close()

	writePacket(t, conn, codec, []byte(payload), inter.CmdDeviceRegister, 1, sessionKey, 2)
	ack := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if len(ack.Payload) != 1 || ack.Payload[0] != byte(inter.RegistrationRejected) {
		t.Fatalf("unexpected refused registration ack: %v", ack.Payload)
	}

	expectClosed(t, conn, codec, sessionKey)
}

func TestHandleConnectionSupportsSessionKeyRotation(t *testing.T) {
	h := newGatewayTestHarness(t)
	meta := inter.DeviceMetadata{
		Name:          "Rekey Device",
		HWVersion:     "v1",
		SWVersion:     "v1",
		ConfigVersion: "v1",
		SerialNumber:  "SN-rekey",
		MACAddress:    "AA:BB:CC:DD:EE:00",
		CreatedAt:     time.Now().UTC(),
	}
	_, token := seedApprovedDevice(t, h, meta)

	conn, codec, sessionKey := startPipeSession(t, h.gateway)
	defer conn.Close()

	writePacket(t, conn, codec, []byte(token), inter.CmdAuthVerify, 1, sessionKey, 2)
	authAck := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if len(authAck.Payload) != 1 || authAck.Payload[0] != 0x00 {
		t.Fatalf("unexpected auth ack payload: %v", authAck.Payload)
	}

	rekeyPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate rekey client key: %v", err)
	}
	writePacket(t, conn, codec, rekeyPriv.PublicKey().Bytes(), inter.CmdKeyExchangeUplink, 1, sessionKey, 3)

	rekeyResp := readAck(t, conn, codec, sessionKey, inter.CmdKeyExchangeDownlink)
	serverPub, err := ecdh.X25519().NewPublicKey(rekeyResp.Payload)
	if err != nil {
		t.Fatalf("failed to parse rekey server key: %v", err)
	}
	nextSessionKey, err := rekeyPriv.ECDH(serverPub)
	if err != nil {
		t.Fatalf("failed to derive next session key: %v", err)
	}

	writePacket(t, conn, codec, nil, inter.CmdHeartbeat, 1, nextSessionKey, 4)
	readAck(t, conn, codec, nextSessionKey, inter.CmdHeartbeat)
}

func TestHandleConnectionReportsDeviceErrorAndCloses(t *testing.T) {
	h := newGatewayTestHarness(t)
	meta := inter.DeviceMetadata{
		Name:          "Error Device",
		HWVersion:     "v1",
		SWVersion:     "v1",
		ConfigVersion: "v1",
		SerialNumber:  "SN-error",
		MACAddress:    "00:AA:11:BB:22:CC",
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

	writePacket(t, conn, codec, []byte("sensor exploded"), inter.CmdErrorReport, 1, sessionKey, 3)
	expectClosed(t, conn, codec, sessionKey)

	level, message := waitForLogRecord(t, h.dbPath, uuid)
	if level != "ERROR" || !strings.Contains(message, "sensor exploded") {
		t.Fatalf("unexpected error log record: level=%s message=%s", level, message)
	}
}

func TestHandleConnectionSupportsAllDownlinkCommands(t *testing.T) {
	h := newGatewayTestHarness(t)
	meta := inter.DeviceMetadata{
		Name:          "Downlink Matrix",
		HWVersion:     "v1",
		SWVersion:     "v1",
		ConfigVersion: "v1",
		SerialNumber:  "SN-downlink-matrix",
		MACAddress:    "FE:DC:BA:98:76:54",
		CreatedAt:     time.Now().UTC(),
	}
	uuid, token := seedApprovedDevice(t, h, meta)

	cases := []struct {
		cmdID   inter.CmdID
		command string
		payload []byte
	}{
		{cmdID: inter.CmdConfigPush, command: "config_push", payload: []byte(`{"sampling":"5s"}`)},
		{cmdID: inter.CmdOtaData, command: "ota_data", payload: []byte{0x01, 0x02, 0x03, 0x04}},
		{cmdID: inter.CmdActionExec, command: "action_exec", payload: []byte(`{"relay":"on"}`)},
		{cmdID: inter.CmdScreenWy, command: "screen_wy", payload: []byte("hello screen")},
	}

	commandIDs := make([]int64, 0, len(cases))
	for _, tc := range cases {
		msg, err := h.downlinkCommands.Enqueue(inter.Scope{}, uuid, tc.cmdID, tc.command, tc.payload)
		if err != nil {
			t.Fatalf("failed to enqueue downlink %v: %v", tc.cmdID, err)
		}
		commandIDs = append(commandIDs, msg.CommandID)
	}

	conn, codec, sessionKey := startPipeSession(t, h.gateway)
	defer conn.Close()

	writePacket(t, conn, codec, []byte(token), inter.CmdAuthVerify, 1, sessionKey, 2)
	authAck := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if len(authAck.Payload) != 1 || authAck.Payload[0] != 0x00 {
		t.Fatalf("unexpected auth ack payload: %v", authAck.Payload)
	}

	for i, tc := range cases {
		packet, err := codec.Unpack(conn, sessionKey)
		if err != nil {
			t.Fatalf("failed to read downlink packet %d: %v", i, err)
		}
		if packet.CmdID != tc.cmdID || packet.IsAck {
			t.Fatalf("unexpected downlink packet %d: %+v", i, packet)
		}
		if !bytes.Equal(packet.Payload, tc.payload) {
			t.Fatalf("unexpected downlink payload %d: got=%v want=%v", i, packet.Payload, tc.payload)
		}
	}

	for i, tc := range cases {
		writeAckPacket(t, conn, codec, nil, tc.cmdID, 1, sessionKey, uint64(10+i))
	}

	time.Sleep(50 * time.Millisecond)
	for i, commandID := range commandIDs {
		waitForCommandStatus(t, h.dbPath, commandID, inter.DeviceCommandStatusAcked)
		if commandID <= 0 {
			t.Fatalf("unexpected command id at index %d: %d", i, commandID)
		}
	}
}

func TestHandleConnectionMarksDownlinkFailedOnDisconnect(t *testing.T) {
	h := newGatewayTestHarness(t)
	meta := inter.DeviceMetadata{
		Name:          "Disconnect Device",
		HWVersion:     "v1",
		SWVersion:     "v1",
		ConfigVersion: "v1",
		SerialNumber:  "SN-disconnect",
		MACAddress:    "12:34:56:78:9A:BC",
		CreatedAt:     time.Now().UTC(),
	}
	uuid, token := seedApprovedDevice(t, h, meta)

	msg, err := h.downlinkCommands.Enqueue(inter.Scope{}, uuid, inter.CmdActionExec, "action_exec", []byte(`{"relay":"off"}`))
	if err != nil {
		t.Fatalf("failed to enqueue downlink command: %v", err)
	}

	conn, codec, sessionKey := startPipeSession(t, h.gateway)

	writePacket(t, conn, codec, []byte(token), inter.CmdAuthVerify, 1, sessionKey, 2)
	authAck := readAck(t, conn, codec, sessionKey, inter.CmdAuthAck)
	if len(authAck.Payload) != 1 || authAck.Payload[0] != 0x00 {
		t.Fatalf("unexpected auth ack payload: %v", authAck.Payload)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("failed to close client connection: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	waitForCommandStatus(t, h.dbPath, msg.CommandID, inter.DeviceCommandStatusFailed)
}

func waitForLogRecord(t *testing.T, dbPath, uuid string) (string, string) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		var level string
		var message string
		err := db.QueryRow("SELECT level, message FROM logs WHERE uuid = ? ORDER BY id DESC LIMIT 1", uuid).Scan(&level, &message)
		if err == nil {
			return level, message
		}
		time.Sleep(10 * time.Millisecond)
	}

	var level string
	var message string
	if err := db.QueryRow("SELECT level, message FROM logs WHERE uuid = ? ORDER BY id DESC LIMIT 1", uuid).Scan(&level, &message); err != nil {
		t.Fatalf("failed to query log record: %v", err)
	}
	return level, message
}
