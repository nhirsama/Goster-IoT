package inter

import (
	"errors"
	"testing"
	"time"
)

func TestLogFieldConstructors(t *testing.T) {
	now := time.Unix(1700000000, 0)
	dur := 3 * time.Second
	err := errors.New("boom")

	cases := []struct {
		name  string
		field LogField
		key   string
		val   any
	}{
		{name: "string", field: String("module", "api"), key: "module", val: "api"},
		{name: "int", field: Int("status", 200), key: "status", val: 200},
		{name: "int64", field: Int64("cost_ms", 12), key: "cost_ms", val: int64(12)},
		{name: "bool", field: Bool("ok", true), key: "ok", val: true},
		{name: "any", field: Any("payload", map[string]int{"a": 1}), key: "payload"},
		{name: "time", field: Time("ts", now), key: "ts", val: now},
		{name: "duration", field: Duration("latency", dur), key: "latency", val: dur},
		{name: "err", field: Err(err), key: "err", val: err},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.field.Key != tc.key {
				t.Fatalf("unexpected key: got %s want %s", tc.field.Key, tc.key)
			}
			if tc.val != nil && tc.field.Value != tc.val {
				t.Fatalf("unexpected value: got %#v want %#v", tc.field.Value, tc.val)
			}
		})
	}
}
