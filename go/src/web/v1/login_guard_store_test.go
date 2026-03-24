package v1

import (
	"errors"
	"testing"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
)

func TestInMemoryLoginAttemptStoreLocksAfterThreshold(t *testing.T) {
	store := NewInMemoryLoginAttemptStore()
	now := time.Unix(1_710_000_000, 0).UTC()
	key := "admin|127.0.0.1"

	if err := store.RecordFailure(key, now, time.Minute, 2*time.Minute, 2); err != nil {
		t.Fatalf("first failure should succeed: %v", err)
	}
	if err := store.RecordFailure(key, now.Add(10*time.Second), time.Minute, 2*time.Minute, 2); err != nil {
		t.Fatalf("second failure should succeed: %v", err)
	}

	snapshot, err := store.Snapshot(key, now.Add(10*time.Second), time.Minute)
	if err != nil {
		t.Fatalf("snapshot should succeed: %v", err)
	}
	if len(snapshot.Failures) != 2 {
		t.Fatalf("expected 2 failures, got %d", len(snapshot.Failures))
	}
	if !snapshot.LockedUntil.After(now.Add(10 * time.Second)) {
		t.Fatalf("expected key to be locked after reaching threshold")
	}
}

func TestInMemoryLoginAttemptStorePrunesExpiredState(t *testing.T) {
	store := NewInMemoryLoginAttemptStore()
	now := time.Unix(1_710_000_000, 0).UTC()
	key := "viewer|127.0.0.1"

	if err := store.RecordFailure(key, now.Add(-2*time.Minute), time.Minute, time.Minute, 3); err != nil {
		t.Fatalf("record failure should succeed: %v", err)
	}

	snapshot, err := store.Snapshot(key, now, time.Minute)
	if err != nil {
		t.Fatalf("snapshot should succeed: %v", err)
	}
	if len(snapshot.Failures) != 0 {
		t.Fatalf("expired failures should be pruned, got %d", len(snapshot.Failures))
	}
	if !snapshot.LockedUntil.IsZero() {
		t.Fatalf("expired state should not remain locked")
	}
}

func TestLoginAttemptGuardPropagatesStoreErrors(t *testing.T) {
	guard := NewLoginAttemptGuardWithStore(appcfg.DefaultWebConfig().LoginProtection, failingLoginAttemptStore{
		snapshotErr: errors.New("snapshot failed"),
		recordErr:   errors.New("record failed"),
		resetErr:    errors.New("reset failed"),
	})

	if _, _, err := guard.Allow("admin", "127.0.0.1:1234"); err == nil {
		t.Fatal("Allow should propagate snapshot error")
	}
	if err := guard.RecordFailure("admin", "127.0.0.1:1234"); err == nil {
		t.Fatal("RecordFailure should propagate store error")
	}
	if err := guard.Reset("admin", "127.0.0.1:1234"); err == nil {
		t.Fatal("Reset should propagate store error")
	}
}

type failingLoginAttemptStore struct {
	snapshotErr error
	recordErr   error
	resetErr    error
}

func (s failingLoginAttemptStore) Snapshot(string, time.Time, time.Duration) (LoginAttemptSnapshot, error) {
	return LoginAttemptSnapshot{}, s.snapshotErr
}

func (s failingLoginAttemptStore) RecordFailure(string, time.Time, time.Duration, time.Duration, int) error {
	return s.recordErr
}

func (s failingLoginAttemptStore) Reset(string) error {
	return s.resetErr
}
