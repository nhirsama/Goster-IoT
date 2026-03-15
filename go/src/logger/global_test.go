package logger

import "testing"

func TestSetDefaultAndDefault(t *testing.T) {
	old := Default()
	t.Cleanup(func() {
		SetDefault(old)
	})

	SetDefault(nil)
	if Default() == nil {
		t.Fatal("default logger should fallback to noop")
	}

	l := NewNoop()
	SetDefault(l)
	if Default() == nil {
		t.Fatal("default logger should not be nil")
	}
}
