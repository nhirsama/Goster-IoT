package web

import (
	"net/http"

	"github.com/aarondl/authboss/v3"
	"github.com/gorilla/sessions"
)

// SessionStorer 适配 gorilla/sessions 到 authboss.ClientStateReadWriter 接口
type SessionStorer struct {
	store sessions.Store
	name  string
}

// NewSessionStorer 创建一个新的 Session 适配器
func NewSessionStorer(name string, store sessions.Store) *SessionStorer {
	return &SessionStorer{store: store, name: name}
}

// ReadState 读取 Session 状态
func (s *SessionStorer) ReadState(r *http.Request) (authboss.ClientState, error) {
	session, err := s.store.Get(r, s.name)
	if err != nil {
		if session == nil {
			return nil, err
		}
	}

	return &SessionState{
		session: session,
		request: r,
	}, nil
}

// WriteState 写入 Session 状态
func (s *SessionStorer) WriteState(w http.ResponseWriter, state authboss.ClientState, events []authboss.ClientStateEvent) error {
	st, ok := state.(*SessionState)
	if !ok {
		return nil
	}

	for _, ev := range events {
		switch ev.Kind {
		case authboss.ClientStateEventPut:
			st.session.Values[ev.Key] = ev.Value
		case authboss.ClientStateEventDel:
			delete(st.session.Values, ev.Key)
		case authboss.ClientStateEventDelAll:
			st.session.Values = make(map[interface{}]interface{})
		}
	}

	return st.session.Save(st.request, w)
}

// SessionState 实现了 authboss.ClientState 接口
type SessionState struct {
	session *sessions.Session
	request *http.Request
}

// Get 获取 Session 值
func (s *SessionState) Get(key string) (string, bool) {
	val, ok := s.session.Values[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}
