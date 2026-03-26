package identity

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondl/authboss/v3"
	"github.com/gorilla/sessions"
)

func TestSessionStorerReadWriteAndGet(t *testing.T) {
	storer := NewSessionStorer("session", sessions.NewCookieStore([]byte("01234567890123456789012345678901")))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	state, err := storer.ReadState(req)
	if err != nil {
		t.Fatalf("ReadState failed: %v", err)
	}

	rec := httptest.NewRecorder()
	if err := storer.WriteState(rec, state, []authboss.ClientStateEvent{
		{Kind: authboss.ClientStateEventPut, Key: "foo", Value: "bar"},
	}); err != nil {
		t.Fatalf("WriteState put failed: %v", err)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range rec.Result().Cookies() {
		req2.AddCookie(cookie)
	}
	state2, err := storer.ReadState(req2)
	if err != nil {
		t.Fatalf("ReadState second failed: %v", err)
	}
	val, ok := state2.(*SessionState).Get("foo")
	if !ok || val != "bar" {
		t.Fatalf("unexpected session value: ok=%v val=%q", ok, val)
	}

	rec2 := httptest.NewRecorder()
	if err := storer.WriteState(rec2, state2, []authboss.ClientStateEvent{
		{Kind: authboss.ClientStateEventDel, Key: "foo"},
		{Kind: authboss.ClientStateEventPut, Key: "bar", Value: "baz"},
		{Kind: authboss.ClientStateEventDelAll},
	}); err != nil {
		t.Fatalf("WriteState delete failed: %v", err)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range rec2.Result().Cookies() {
		req3.AddCookie(cookie)
	}
	state3, err := storer.ReadState(req3)
	if err != nil {
		t.Fatalf("ReadState third failed: %v", err)
	}
	if val, ok := state3.(*SessionState).Get("bar"); ok || val != "" {
		t.Fatalf("expected cleared session state, got ok=%v val=%q", ok, val)
	}
}

func TestSessionStorerWriteStateIgnoresUnknownState(t *testing.T) {
	storer := NewSessionStorer("session", sessions.NewCookieStore([]byte("01234567890123456789012345678901")))
	if err := storer.WriteState(httptest.NewRecorder(), authboss.ClientState(nil), nil); err != nil {
		t.Fatalf("WriteState should ignore unknown state: %v", err)
	}
}
