package device_manager

import (
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestEvaluateAccessControl(t *testing.T) {
	tests := []struct {
		name       string
		points     []inter.MetricPoint
		wantA      *int
		wantB      *int
		wantOpen   *bool
		wantStatus string
		wantTS     *int64
	}{
		{
			name: "both high opens door",
			points: []inter.MetricPoint{
				{Timestamp: 1000, Value: 1, Type: MetricTypeAccessSignalA},
				{Timestamp: 1200, Value: 1, Type: MetricTypeAccessSignalB},
			},
			wantA:      intPtr(1),
			wantB:      intPtr(1),
			wantOpen:   boolPtr(true),
			wantStatus: "open",
			wantTS:     int64Ptr(1200),
		},
		{
			name: "any low closes door",
			points: []inter.MetricPoint{
				{Timestamp: 1000, Value: 1, Type: MetricTypeAccessSignalA},
				{Timestamp: 1200, Value: 0, Type: MetricTypeAccessSignalB},
			},
			wantA:      intPtr(1),
			wantB:      intPtr(0),
			wantOpen:   boolPtr(false),
			wantStatus: "closed",
			wantTS:     int64Ptr(1200),
		},
		{
			name: "latest signal wins",
			points: []inter.MetricPoint{
				{Timestamp: 1000, Value: 1, Type: MetricTypeAccessSignalA},
				{Timestamp: 1100, Value: 1, Type: MetricTypeAccessSignalB},
				{Timestamp: 1300, Value: 0, Type: MetricTypeAccessSignalA},
			},
			wantA:      intPtr(0),
			wantB:      intPtr(1),
			wantOpen:   boolPtr(false),
			wantStatus: "closed",
			wantTS:     int64Ptr(1300),
		},
		{
			name: "missing signal is unknown",
			points: []inter.MetricPoint{
				{Timestamp: 1000, Value: 1, Type: MetricTypeAccessSignalA},
			},
			wantA:      intPtr(1),
			wantB:      nil,
			wantOpen:   nil,
			wantStatus: "unknown",
			wantTS:     int64Ptr(1000),
		},
		{
			name:       "no signals is unknown",
			points:     []inter.MetricPoint{{Timestamp: 1000, Value: 23, Type: 1}},
			wantStatus: "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateAccessControl(tc.points)
			if !sameIntPtr(got.SignalA, tc.wantA) || !sameIntPtr(got.SignalB, tc.wantB) {
				t.Fatalf("unexpected signals: got A=%v B=%v want A=%v B=%v", ptrValue(got.SignalA), ptrValue(got.SignalB), ptrValue(tc.wantA), ptrValue(tc.wantB))
			}
			if !sameBoolPtr(got.Open, tc.wantOpen) {
				t.Fatalf("unexpected open: got %v want %v", ptrValue(got.Open), ptrValue(tc.wantOpen))
			}
			if got.StatusText != tc.wantStatus {
				t.Fatalf("unexpected status: got %q want %q", got.StatusText, tc.wantStatus)
			}
			if !sameInt64Ptr(got.EvaluatedAtMs, tc.wantTS) {
				t.Fatalf("unexpected evaluated_at_ms: got %v want %v", ptrValue(got.EvaluatedAtMs), ptrValue(tc.wantTS))
			}
		})
	}
}

func intPtr(v int) *int { return &v }

func int64Ptr(v int64) *int64 { return &v }

func boolPtr(v bool) *bool { return &v }

func sameIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func sameInt64Ptr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func sameBoolPtr(a, b *bool) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func ptrValue[T any](v *T) any {
	if v == nil {
		return nil
	}
	return *v
}
