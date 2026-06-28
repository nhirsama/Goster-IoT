package iot_gateway

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
)

type noopGatewayBackend struct{}

func (noopGatewayBackend) AuthenticateDevice(string) (string, error) {
	return "", nil
}

func (noopGatewayBackend) RegisterDevice(inter.DeviceMetadata) (inter.DeviceRegistrationResult, error) {
	return inter.DeviceRegistrationResult{}, nil
}

func (noopGatewayBackend) ReportHeartbeat(string) error {
	return nil
}

func (noopGatewayBackend) ReportMetrics(string, []inter.MetricPoint) error {
	return nil
}

func (noopGatewayBackend) ReportLog(string, inter.LogUploadData) error {
	return nil
}

func (noopGatewayBackend) ReportEvent(string, []byte) error {
	return nil
}

func (noopGatewayBackend) ReportDeviceError(string, []byte) error {
	return nil
}

func (noopGatewayBackend) PopDownlink(string) (inter.DownlinkMessage, bool, error) {
	return inter.DownlinkMessage{}, false, nil
}

func (noopGatewayBackend) RequeueDownlink(string, inter.DownlinkMessage) error {
	return nil
}

func (noopGatewayBackend) MarkDownlinkSent(int64) error {
	return nil
}

func (noopGatewayBackend) MarkDownlinkAcked(int64) error {
	return nil
}

func (noopGatewayBackend) MarkDownlinkFailed(int64, string) error {
	return nil
}

func TestGatewayStartStopsOnContextCancel(t *testing.T) {
	listener := newGatewayListener(t)
	addr := listener.Addr().String()
	svc := NewGatewayWithConfig(noopGatewayBackend{}, logger.NewNoop(), appcfg.APIConfig{
		ReadTimeout:           5 * time.Second,
		RegisterAckGraceDelay: 5 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- svc.Serve(ctx, listener)
	}()

	waitForGatewayTCPServer(t, addr)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("failed to dial running gateway: %v", err)
	}
	defer conn.Close()

	cancel()

	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("failed to set read deadline: %v", err)
	}
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err == nil {
		t.Fatal("expected connection to be closed after context cancel")
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatalf("expected connection close, got timeout: %v", err)
	}
	if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
		t.Fatalf("unexpected read error after cancel: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("gateway should stop cleanly on context cancel: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("gateway did not stop after context cancel")
	}
}

func newGatewayListener(t *testing.T) net.Listener {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("tcp listener is unavailable in current environment: %v", err)
	}
	return l
}

func waitForGatewayTCPServer(t *testing.T, address string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("gateway tcp server did not start at %s", address)
}
