package v1_test

import (
	"net/http/httptest"
	"testing"

	apiv1 "github.com/nhirsama/Goster-IoT/src/web/v1"
)

func TestParsePositiveIntQuery(t *testing.T) {
	v, err := apiv1.ParsePositiveIntQuery("", 7, 0)
	if err != nil || v != 7 {
		t.Fatalf("empty value should fallback to default, got v=%d err=%v", v, err)
	}

	if _, err := apiv1.ParsePositiveIntQuery("abc", 1, 0); err == nil {
		t.Fatalf("non-integer value should return error")
	}

	if _, err := apiv1.ParsePositiveIntQuery("0", 1, 0); err == nil {
		t.Fatalf("zero value should return error")
	}

	if _, err := apiv1.ParsePositiveIntQuery("1001", 1, 1000); err == nil {
		t.Fatalf("value above max should return error")
	}
}

func TestResolveMetricsRangeWithExplicitWindow(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/metrics/dev-1?range=all&start_ms=1&end_ms=2000000000000", nil)
	const minValidMetricsTs int64 = 1672531200000
	start, end, label, err := apiv1.ResolveMetricsRange(req, minValidMetricsTs, "1h")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if label != "all" {
		t.Fatalf("unexpected range label: %s", label)
	}
	if start != minValidMetricsTs {
		t.Fatalf("start should be clamped to minValidMetricsTimestampMs, got %d", start)
	}
	if end != 2000000000000 {
		t.Fatalf("unexpected end: %d", end)
	}
}

func TestResolveMetricsRangeInvalidQueries(t *testing.T) {
	const minValidMetricsTs int64 = 1672531200000
	startOnly := httptest.NewRequest("GET", "/api/v1/metrics/dev-1?start_ms=1700000000000", nil)
	if _, _, _, err := apiv1.ResolveMetricsRange(startOnly, minValidMetricsTs, "1h"); err == nil {
		t.Fatalf("start_ms without end_ms should return error")
	}

	invalidRange := httptest.NewRequest("GET", "/api/v1/metrics/dev-1?range=30m", nil)
	if _, _, _, err := apiv1.ResolveMetricsRange(invalidRange, minValidMetricsTs, "1h"); err == nil {
		t.Fatalf("invalid range should return error")
	}

	invalidRangeWithWindow := httptest.NewRequest("GET", "/api/v1/metrics/dev-1?range=30m&start_ms=1700000000000&end_ms=1700003600000", nil)
	if _, _, _, err := apiv1.ResolveMetricsRange(invalidRangeWithWindow, minValidMetricsTs, "1h"); err == nil {
		t.Fatalf("invalid range should return error even with explicit window")
	}
}

func TestParseDownlinkCommand(t *testing.T) {
	cmdID, name, err := apiv1.ParseDownlinkCommand(" action_exec ")
	if err != nil {
		t.Fatalf("parseDownlinkCommand should parse valid command: %v", err)
	}
	if cmdID != 0x0203 || name != "action_exec" {
		t.Fatalf("unexpected parse result: cmdID=%d name=%s", cmdID, name)
	}

	if _, _, err := apiv1.ParseDownlinkCommand("reboot"); err == nil {
		t.Fatal("parseDownlinkCommand should reject unsupported command")
	}
}
