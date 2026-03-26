package external_test

import (
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/storage/external"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/testhelper"
)

func TestRepositoryExternalEntityAndObservationFlow(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "external_repo.db")
	repo := external.NewRepository(base.DB)

	state := 23.5
	text := "ok"
	flag := true
	entity := inter.ExternalEntity{
		Source:      "ha",
		EntityID:    "sensor.temp_1",
		Domain:      "sensor",
		Name:        "Temp 1",
		ValueType:   "number",
		LastStateTS: time.Now().UnixMilli(),
		LastNum:     &state,
		LastText:    &text,
		LastBool:    &flag,
		Attributes:  map[string]interface{}{"room": "lab"},
	}
	if err := repo.UpsertExternalEntity(entity); err != nil {
		t.Fatalf("UpsertExternalEntity failed: %v", err)
	}

	got, err := repo.GetExternalEntity("ha", "sensor.temp_1")
	if err != nil {
		t.Fatalf("GetExternalEntity failed: %v", err)
	}
	if got.Name != "Temp 1" || got.Attributes["room"] != "lab" {
		t.Fatalf("unexpected external entity: %+v", got)
	}

	observedAt := time.Now().UnixMilli()
	if err := repo.BatchAppendExternalObservations([]inter.ExternalObservation{
		{
			Source:    "ha",
			EntityID:  "sensor.temp_1",
			Timestamp: observedAt,
			ValueNum:  &state,
			Unit:      "C",
		},
		{
			Source:    "ha",
			EntityID:  "sensor.temp_1",
			Timestamp: observedAt,
			ValueNum:  &state,
			Unit:      "C",
		},
	}); err != nil {
		t.Fatalf("BatchAppendExternalObservations failed: %v", err)
	}

	items, err := repo.QueryExternalObservations("ha", "sensor.temp_1", observedAt-1000, observedAt+1000, 10)
	if err != nil {
		t.Fatalf("QueryExternalObservations failed: %v", err)
	}
	if len(items) != 1 || items[0].Unit != "C" {
		t.Fatalf("unexpected observations: %+v", items)
	}

	list, err := repo.ListExternalEntities("ha", "sensor", 10, 0)
	if err != nil {
		t.Fatalf("ListExternalEntities failed: %v", err)
	}
	if len(list) != 1 || list[0].EntityID != "sensor.temp_1" {
		t.Fatalf("unexpected external entity list: %+v", list)
	}
}
