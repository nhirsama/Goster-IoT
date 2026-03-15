package logger

import (
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestRedactFields(t *testing.T) {
	fields := []inter.LogField{
		inter.String("token", "abc"),
		inter.String("api_key", "def"),
		inter.String("username", "alice"),
	}

	got := RedactFields(fields...)
	if len(got) != len(fields) {
		t.Fatalf("unexpected size: got=%d want=%d", len(got), len(fields))
	}

	if got[0].Value != redactedValue {
		t.Fatalf("token should be redacted: %#v", got[0].Value)
	}
	if got[1].Value != redactedValue {
		t.Fatalf("api_key should be redacted: %#v", got[1].Value)
	}
	if got[2].Value != "alice" {
		t.Fatalf("username should keep original: %#v", got[2].Value)
	}
}
