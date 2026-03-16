package web

import (
	"net/http/httptest"
	"testing"
)

func TestParsePositiveIntQuery(t *testing.T) {
	v, err := parsePositiveIntQuery("", 7, 0)
	if err != nil || v != 7 {
		t.Fatalf("empty value should fallback to default, got v=%d err=%v", v, err)
	}

	if _, err := parsePositiveIntQuery("abc", 1, 0); err == nil {
		t.Fatalf("non-integer value should return error")
	}

	if _, err := parsePositiveIntQuery("0", 1, 0); err == nil {
		t.Fatalf("zero value should return error")
	}

	if _, err := parsePositiveIntQuery("1001", 1, 1000); err == nil {
		t.Fatalf("value above max should return error")
	}
}

func TestResolveMetricsRangeWithExplicitWindow(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/metrics/dev-1?range=all&start_ms=1&end_ms=2000000000000", nil)
	const minValidMetricsTs int64 = 1672531200000
	start, end, label, err := resolveMetricsRange(req, minValidMetricsTs, "1h")
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
	if _, _, _, err := resolveMetricsRange(startOnly, minValidMetricsTs, "1h"); err == nil {
		t.Fatalf("start_ms without end_ms should return error")
	}

	invalidRange := httptest.NewRequest("GET", "/api/v1/metrics/dev-1?range=30m", nil)
	if _, _, _, err := resolveMetricsRange(invalidRange, minValidMetricsTs, "1h"); err == nil {
		t.Fatalf("invalid range should return error")
	}

	invalidRangeWithWindow := httptest.NewRequest("GET", "/api/v1/metrics/dev-1?range=30m&start_ms=1700000000000&end_ms=1700003600000", nil)
	if _, _, _, err := resolveMetricsRange(invalidRangeWithWindow, minValidMetricsTs, "1h"); err == nil {
		t.Fatalf("invalid range should return error even with explicit window")
	}
}
