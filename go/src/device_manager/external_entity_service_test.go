package device_manager

import (
	"path/filepath"
	"testing"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestExternalEntityServiceNormalizesAndQueriesEntities(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "external.db")
	ds, err := datastore.NewDataStoreSql(dbPath)
	if err != nil {
		t.Fatalf("failed to init datastore: %v", err)
	}

	service := NewExternalEntityService(ds, appcfg.DefaultDeviceManagerConfig())
	entity := inter.ExternalEntity{
		Source:    "  mijia ",
		EntityID:  " sensor.temp ",
		Domain:    " sensor ",
		ValueType: "",
		Name:      "温度传感器",
	}
	if err := service.UpsertExternalEntity(entity); err != nil {
		t.Fatalf("upsert external entity failed: %v", err)
	}

	items, err := service.ListExternalEntities("mijia", "sensor", 1, 10)
	if err != nil {
		t.Fatalf("list external entities failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected external entity count: got %d want 1", len(items))
	}
	if items[0].GosterUUID == "" || items[0].ValueType != "string" {
		t.Fatalf("unexpected normalized external entity: %+v", items[0])
	}

	value := 23.5
	if err := service.BatchAppendExternalObservations([]inter.ExternalObservation{
		{
			Source:    "mijia",
			EntityID:  "sensor.temp",
			Timestamp: time.Now().UnixMilli(),
			ValueNum:  &value,
			Unit:      "C",
		},
	}); err != nil {
		t.Fatalf("append external observation failed: %v", err)
	}

	results, err := service.QueryExternalObservations("mijia", "sensor.temp", 0, time.Now().UnixMilli()+1000, 10)
	if err != nil {
		t.Fatalf("query external observations failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("unexpected observation count: got %d want 1", len(results))
	}
}
