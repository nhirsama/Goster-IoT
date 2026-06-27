package datastore

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

const testCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func newTestStore(t *testing.T) inter.DataStore {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping database test")
	}

	store, err := NewDataStoreSql(dbURL)
	if err != nil {
		t.Fatalf("NewDataStoreSql failed: %v", err)
	}

	// Clean up test data before and after test
	sqlStore := store.(*DataStoreSql)
	cleanupTables := func() {
		tables := []string{
			"logs", "metrics", "integration_external_observations",
			"integration_external_entities", "integration_external_commands",
			"group_devices", "group_users", "device_groups",
			"devices", "tenant_users", "users",
		}
		for _, table := range tables {
			sqlStore.db.Exec("DELETE FROM " + table + " WHERE created_at > NOW() - INTERVAL '1 hour'")
		}
	}

	cleanupTables()

	t.Cleanup(func() {
		cleanupTables()
	})

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
