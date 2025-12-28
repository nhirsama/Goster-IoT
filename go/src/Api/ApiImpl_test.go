package Api

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/binary"
	"math"
	"net"
	"os"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/DataStore"
	"github.com/nhirsama/Goster-IoT/src/DeviceManager"
	"github.com/nhirsama/Goster-IoT/src/IdentityManager"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to wait for server port
func waitForServer(t *testing.T, address string) {
	for i := 0; i < 20; i++ {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("Server failed to start on %s", address)
}

func TestComprehensiveFlow(t *testing.T) {
	// 1. Setup Environment
	dbFile := "test_comprehensive.db"
	os.Remove(dbFile)
	defer os.Remove(dbFile)

	ds, err := DataStore.NewDataStoreSql(dbFile)
	require.NoError(t, err)
	im := IdentityManager.NewIdentityManager(ds)
	dm := DeviceManager.NewDeviceManager(ds, im)
	api := NewApi(ds, dm, im)

	// 2. Start Server
	go api.Start()
	waitForServer(t, "127.0.0.1:8081")

	// 3. Register Device
	meta := inter.DeviceMetadata{Name: "TestDev", SerialNumber: "SN123", MACAddress: "11:22:33"}
	uuid := im.GenerateUUID(meta)
	token, err := im.RegisterDevice(uuid, meta)
	require.NoError(t, err)

	// =========================================================================
	// Scenario A: Happy Path (Handshake -> Auth -> Metrics -> Log)
	// =========================================================================
	t.Run("HappyPath", func(t *testing.T) {
		conn, err := net.Dial("tcp", "127.0.0.1:8081")
		require.NoError(t, err)
		defer conn.Close()

		codec := protocol.NewGosterCodec()
		privKey, _ := ecdh.X25519().GenerateKey(rand.Reader)

		// 1. Handshake
		pubKey := privKey.PublicKey().Bytes()
		buf, _ := codec.Pack(pubKey, inter.CmdHandshakeInit, 0, nil, 1)
		conn.Write(buf)

		resp, err := codec.Unpack(conn, nil)
		require.NoError(t, err)
		assert.Equal(t, inter.CmdHandshakeResp, resp.CmdID)

		serverPub, err := ecdh.X25519().NewPublicKey(resp.Payload)
		require.NoError(t, err)
		sessionKey, err := privKey.ECDH(serverPub)
		require.NoError(t, err)

		// 2. Auth
		authBuf, _ := codec.Pack([]byte(token), inter.CmdAuthVerify, 0, sessionKey, 2)
		conn.Write(authBuf)

		ack, err := codec.Unpack(conn, sessionKey)
		require.NoError(t, err)
		assert.Equal(t, inter.CmdAuthAck, ack.CmdID)
		assert.Equal(t, byte(0), ack.Payload[0], "Auth should succeed")

		// 3. Metrics
		blob := make([]byte, 4)
		binary.LittleEndian.PutUint32(blob, math.Float32bits(123.456))

		mPayload := make([]byte, 0)
		tsBuf := make([]byte, 8)
		binary.LittleEndian.PutUint64(tsBuf, uint64(time.Now().UnixMilli()))
		mPayload = append(mPayload, tsBuf...)
		intervalBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(intervalBuf, 1000)
		mPayload = append(mPayload, intervalBuf...)
		mPayload = append(mPayload, 0)
		cntBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(cntBuf, 1)
		mPayload = append(mPayload, cntBuf...)
		mPayload = append(mPayload, blob...)

		mBuf, _ := codec.Pack(mPayload, inter.CmdMetricsReport, 1, sessionKey, 3)
		conn.Write(mBuf)

		// 4. Log
		lPayload := make([]byte, 0)
		binary.LittleEndian.PutUint64(tsBuf, uint64(time.Now().UnixMilli()))
		lPayload = append(lPayload, tsBuf...)
		lPayload = append(lPayload, byte(inter.LogLevelWarn))
		msg := "Test Log Message"
		lLenBuf := make([]byte, 2)
		binary.LittleEndian.PutUint16(lLenBuf, uint16(len(msg)))
		lPayload = append(lPayload, lLenBuf...)
		lPayload = append(lPayload, []byte(msg)...)

		lBuf, _ := codec.Pack(lPayload, inter.CmdLogReport, 1, sessionKey, 4)
		conn.Write(lBuf)

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		// Verify DB
		points, err := ds.QueryMetrics(uuid, 0, time.Now().UnixMilli()+1000)
		require.NoError(t, err)
		if assert.NotEmpty(t, points) {
			assert.InDelta(t, 123.456, points[len(points)-1].Value, 0.001)
		}
	})

	// =========================================================================
	// Scenario B: Auth Failure
	// =========================================================================
	t.Run("AuthFailure", func(t *testing.T) {
		conn, err := net.Dial("tcp", "127.0.0.1:8081")
		require.NoError(t, err)
		defer conn.Close()

		codec := protocol.NewGosterCodec()
		privKey, _ := ecdh.X25519().GenerateKey(rand.Reader)

		// Handshake
		pubKey := privKey.PublicKey().Bytes()
		buf, _ := codec.Pack(pubKey, inter.CmdHandshakeInit, 0, nil, 1)
		conn.Write(buf)
		resp, err := codec.Unpack(conn, nil)
		require.NoError(t, err)
		serverPub, _ := ecdh.X25519().NewPublicKey(resp.Payload)
		sessionKey, _ := privKey.ECDH(serverPub)

		// Auth with BAD TOKEN
		authBuf, _ := codec.Pack([]byte("bad_token"), inter.CmdAuthVerify, 0, sessionKey, 2)
		conn.Write(authBuf)

		// Expect Fail Ack OR immediate close
		ack, err := codec.Unpack(conn, sessionKey)
		if err == nil {
			assert.Equal(t, inter.CmdAuthAck, ack.CmdID)
			assert.Equal(t, byte(1), ack.Payload[0], "Auth should fail")

			// Should close after
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			_, err = codec.Unpack(conn, sessionKey)
			assert.Error(t, err)
		} else {
			// If it errored out (EOF), it's also a sign of closure
			assert.Error(t, err)
		}
	})

	// =========================================================================
	// Scenario C: Unauthenticated Access
	// =========================================================================
	t.Run("UnauthAccess", func(t *testing.T) {
		conn, err := net.Dial("tcp", "127.0.0.1:8081")
		require.NoError(t, err)
		defer conn.Close()

		codec := protocol.NewGosterCodec()
		privKey, _ := ecdh.X25519().GenerateKey(rand.Reader)
		pubKey := privKey.PublicKey().Bytes()
		buf, _ := codec.Pack(pubKey, inter.CmdHandshakeInit, 0, nil, 1)
		conn.Write(buf)
		resp, _ := codec.Unpack(conn, nil)
		serverPub, _ := ecdh.X25519().NewPublicKey(resp.Payload)
		sessionKey, _ := privKey.ECDH(serverPub)

		// Skip Auth, Send Metrics
		mBuf, _ := codec.Pack(make([]byte, 20), inter.CmdMetricsReport, 1, sessionKey, 2)
		conn.Write(mBuf)

		// Server should close connection
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		_, err = codec.Unpack(conn, sessionKey)
		assert.Error(t, err)
	})
}
