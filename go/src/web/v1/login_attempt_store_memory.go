package v1

import (
	"sync"
	"time"
)

// InMemoryLoginAttemptStore 是 LoginAttemptStore 的进程内实现。
// 该实现适合单实例部署，也作为后续替换共享存储的默认回退。
type InMemoryLoginAttemptStore struct {
	mu       sync.Mutex
	attempts map[string]LoginAttemptSnapshot
}

// NewInMemoryLoginAttemptStore 创建默认的内存态登录尝试存储。
func NewInMemoryLoginAttemptStore() LoginAttemptStore {
	return &InMemoryLoginAttemptStore{
		attempts: make(map[string]LoginAttemptSnapshot),
	}
}

func (s *InMemoryLoginAttemptStore) Snapshot(key string, now time.Time, window time.Duration) (LoginAttemptSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.prunedState(key, now, window)
	return cloneLoginAttemptSnapshot(state), nil
}

func (s *InMemoryLoginAttemptStore) RecordFailure(key string, now time.Time, window, lockout time.Duration, maxFailures int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.prunedState(key, now, window)
	state.Failures = append(state.Failures, now)
	if len(state.Failures) >= maxFailures {
		state.LockedUntil = now.Add(lockout)
	}
	s.attempts[key] = state
	return nil
}

func (s *InMemoryLoginAttemptStore) Reset(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.attempts, key)
	return nil
}

func (s *InMemoryLoginAttemptStore) prunedState(key string, now time.Time, window time.Duration) LoginAttemptSnapshot {
	state := s.attempts[key]
	cutoff := now.Add(-window)
	kept := state.Failures[:0]
	for _, ts := range state.Failures {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	state.Failures = kept
	if !state.LockedUntil.After(now) {
		state.LockedUntil = time.Time{}
	}
	if len(state.Failures) == 0 && state.LockedUntil.IsZero() {
		delete(s.attempts, key)
		return LoginAttemptSnapshot{}
	}
	s.attempts[key] = state
	return state
}

func cloneLoginAttemptSnapshot(state LoginAttemptSnapshot) LoginAttemptSnapshot {
	cloned := LoginAttemptSnapshot{
		LockedUntil: state.LockedUntil,
	}
	if len(state.Failures) > 0 {
		cloned.Failures = append(make([]time.Time, 0, len(state.Failures)), state.Failures...)
	}
	return cloned
}
