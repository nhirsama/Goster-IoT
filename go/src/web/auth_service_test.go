package web

import "testing"

func TestNewAuthServiceRejectsNilAuthboss(t *testing.T) {
	if _, err := NewAuthService(nil); err == nil {
		t.Fatal("expected nil authboss error")
	}
}
