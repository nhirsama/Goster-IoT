package web

import (
	"net/http/httptest"
	"testing"

	"github.com/aarondl/authboss/v3"
	"github.com/gorilla/sessions"
)

func TestSessionStorerReadWriteState(t *testing.T) {
	storer := NewSessionStorer("goster", sessions.NewCookieStore([]byte("secret-key-1234567890")))
	req := httptest.NewRequest("GET", "/", nil)

	state, err := storer.ReadState(req)
	if err != nil {
		t.Fatalf("ReadState failed: %v", err)
	}

	rec := httptest.NewRecorder()
	err = storer.WriteState(rec, state, []authboss.ClientStateEvent{
		{Kind: authboss.ClientStateEventPut, Key: "user", Value: "alice"},
	})
	if err != nil {
		t.Fatalf("WriteState put failed: %v", err)
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("expected session cookie to be written")
	}

	nextReq := httptest.NewRequest("GET", "/", nil)
	for _, cookie := range rec.Result().Cookies() {
		nextReq.AddCookie(cookie)
	}
	nextState, err := storer.ReadState(nextReq)
	if err != nil {
		t.Fatalf("ReadState with cookie failed: %v", err)
	}
	got, ok := nextState.Get("user")
	if !ok || got != "alice" {
		t.Fatalf("unexpected session state: ok=%v value=%s", ok, got)
	}

	delRec := httptest.NewRecorder()
	err = storer.WriteState(delRec, nextState, []authboss.ClientStateEvent{
		{Kind: authboss.ClientStateEventDel, Key: "user"},
	})
	if err != nil {
		t.Fatalf("WriteState delete failed: %v", err)
	}

	delReq := httptest.NewRequest("GET", "/", nil)
	for _, cookie := range delRec.Result().Cookies() {
		delReq.AddCookie(cookie)
	}
	clearedState, err := storer.ReadState(delReq)
	if err != nil {
		t.Fatalf("ReadState after delete failed: %v", err)
	}
	if _, ok := clearedState.Get("user"); ok {
		t.Fatal("expected deleted session key to be absent")
	}
}

func TestSessionStorerWriteStateIgnoresForeignState(t *testing.T) {
	storer := NewSessionStorer("goster", sessions.NewCookieStore([]byte("secret-key-1234567890")))
	rec := httptest.NewRecorder()
	if err := storer.WriteState(rec, nil, nil); err != nil {
		t.Fatalf("WriteState should ignore foreign state: %v", err)
	}
}
