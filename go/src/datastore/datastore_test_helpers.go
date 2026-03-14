package datastore

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

const testCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func newTestStore(t *testing.T) inter.DataStore {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "datastore_test.db")
	store, err := NewDataStoreSql(dbPath)
	if err != nil {
		t.Fatalf("NewDataStoreSql failed: %v", err)
	}
	return store
}

func asSQLStore(t *testing.T, store inter.DataStore) *DataStoreSql {
	t.Helper()

	sqlStore, ok := store.(*DataStoreSql)
	if !ok {
		t.Fatalf("unexpected datastore concrete type: %T", store)
	}
	return sqlStore
}

func randomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = testCharset[rand.Intn(len(testCharset))]
	}
	return string(b)
}

func generateUUID(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), rand.Int63())
}

func generateRandomMeta() inter.DeviceMetadata {
	return inter.DeviceMetadata{
		Name:               "Device-" + randomString(6),
		HWVersion:          "hw-" + randomString(3),
		SWVersion:          "sw-" + randomString(3),
		ConfigVersion:      randomString(8),
		SerialNumber:       "SN-" + randomString(12),
		MACAddress:         fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", rand.Intn(255), rand.Intn(255), rand.Intn(255), rand.Intn(255), rand.Intn(255), rand.Intn(255)),
		CreatedAt:          time.Now().UTC().Truncate(time.Second),
		Token:              "tk-" + randomString(24),
		AuthenticateStatus: inter.AuthenticatePending,
	}
}
