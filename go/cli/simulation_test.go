package cli

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/protocol"
)

type cliAPIEnvelope struct {
	Code int                    `json:"code"`
	Data map[string]interface{} `json:"data"`
}

func reserveTCPAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve tcp address: %v", err)
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

func waitForHTTPServer(t *testing.T, url string) *http.Response {
	t.Helper()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			return resp
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("http server did not start at %s", url)
	return nil
}

func TestCLIStartBootstrapsHTTPAndTCP(t *testing.T) {
	webAddr := reserveTCPAddr(t)
	apiAddr := reserveTCPAddr(t)

	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "cli_test.db"))
	t.Setenv("WEB_HTTP_ADDR", webAddr)
	t.Setenv("API_TCP_ADDR", apiAddr)
	t.Setenv("AUTHBOSS_ROOT_URL", "http://"+webAddr)

	if err := RunWithArgs(context.Background(), []string{"db", "init"}); err != nil {
		t.Fatalf("db init failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go start(ctx)

	resp := waitForHTTPServer(t, "http://"+webAddr+"/api/v1/auth/captcha/config")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected captcha config status: got %d want %d", resp.StatusCode, http.StatusOK)
	}

	var env cliAPIEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("failed to decode captcha response: %v", err)
	}
	if env.Code != 0 {
		t.Fatalf("unexpected captcha response code: got %d want 0", env.Code)
	}

	waitForTCPServer(t, apiAddr)

	conn, err := net.Dial("tcp", apiAddr)
	if err != nil {
		t.Fatalf("failed to dial api tcp server: %v", err)
	}
	defer conn.Close()

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

	packet, err := codec.Unpack(conn, nil)
	if err != nil {
		t.Fatalf("failed to read handshake response: %v", err)
	}
	if packet.CmdID != inter.CmdHandshakeResp {
		t.Fatalf("unexpected handshake response cmd: got %v want %v", packet.CmdID, inter.CmdHandshakeResp)
	}
	if !packet.IsAck {
		t.Fatal("handshake response should be ack")
	}
	if len(packet.Payload) != 32 {
		t.Fatalf("unexpected server public key length: got %d want 32", len(packet.Payload))
	}
}
