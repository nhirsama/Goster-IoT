package tests

import (
	"net"
	"net/http"
	"testing"
	"time"
)

func reserveTCPAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("tcp listener is unavailable in current environment: %v", err)
	}
	defer l.Close()
	return l.Addr().String()
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
