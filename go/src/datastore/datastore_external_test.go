package datastore

import (
	"database/sql"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestExternalEntityCRUD(t *testing.T) {
	store := newTestStore(t)

	initialNum := 12.3
	entity := inter.ExternalEntity{
		Source:      "mijia",
		EntityID:    "switch.plug_living_room",
		Domain:      "switch",
		GosterUUID:  generateUUID("goster"),
		DeviceID:    "did-123",
		Model:       "chuangmi.plug.v3",
		Name:        "Living Plug",
		RoomName:    "Living Room",
		Unit:        "",
		ValueType:   "bool",
		DeviceClass: "outlet",
		Attributes: map[string]interface{}{
			"online": true,
			"rssi":   -45,
		},
		LastStateTS: time.Now().UnixMilli(),
		LastNum:     &initialNum,
	}

	if err := store.UpsertExternalEntity(entity); err != nil {
		t.Fatalf("UpsertExternalEntity failed: %v", err)
	}

	got, err := store.GetExternalEntity(entity.Source, entity.EntityID)
	if err != nil {
		t.Fatalf("GetExternalEntity failed: %v", err)
	}
	if got.EntityID != entity.EntityID || got.Domain != entity.Domain {
		t.Fatalf("GetExternalEntity mismatch: %+v", got)
	}
	if got.Attributes["online"] != true {
		t.Fatalf("attributes not persisted: %+v", got.Attributes)
	}

	updatedText := "on"
	entity.LastText = &updatedText
	entity.LastNum = nil
	entity.LastStateTS = time.Now().UnixMilli()
	if err := store.UpsertExternalEntity(entity); err != nil {
		t.Fatalf("UpsertExternalEntity update failed: %v", err)
	}

	list, err := store.ListExternalEntities("mijia", "switch", 20, 0)
	if err != nil {
		t.Fatalf("ListExternalEntities failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListExternalEntities count mismatch: got=%d want=1", len(list))
	}
}

func TestExternalObservationBatchAndQuery(t *testing.T) {
	store := newTestStore(t)
	ts := time.Now().UnixMilli()

	num := 23.5
	on := true
	text := "on"
	items := []inter.ExternalObservation{
		{
			Source:    "mijia",
			EntityID:  "sensor.plug_power",
			Timestamp: ts,
			ValueNum:  &num,
			Unit:      "W",
		},
		{
			Source:    "mijia",
			EntityID:  "sensor.plug_power",
			Timestamp: ts,
			ValueNum:  &num,
			Unit:      "W",
		},
		{
			Source:    "mijia",
			EntityID:  "sensor.plug_power",
			Timestamp: ts,
			ValueBool: &on,
		},
		{
			Source:    "mijia",
			EntityID:  "switch.plug_main",
			Timestamp: ts + 1,
			ValueText: &text,
		},
	}
	if err := store.BatchAppendExternalObservations(items); err != nil {
		t.Fatalf("BatchAppendExternalObservations failed: %v", err)
	}

	results, err := store.QueryExternalObservations("mijia", "sensor.plug_power", ts-10, ts+10, 20)
	if err != nil {
		t.Fatalf("QueryExternalObservations failed: %v", err)
	}
	// duplicate num observation should be ignored by UNIQUE(source, entity_id, ts, value_sig)
	if len(results) != 2 {
		t.Fatalf("QueryExternalObservations dedupe mismatch: got=%d want=2", len(results))
	}
}

func TestDeviceCommandLogLifecycle(t *testing.T) {
	store := newTestStore(t)
	sqlStore := asSQLStore(t, store)

	commandID, err := store.CreateDeviceCommand("device-1", inter.CmdActionExec, "action_exec", []byte(`{"op":"reboot"}`))
	if err != nil {
		t.Fatalf("CreateDeviceCommand failed: %v", err)
	}
	if commandID <= 0 {
		t.Fatalf("unexpected command id: %d", commandID)
	}

	if err := store.UpdateDeviceCommandStatus(commandID, inter.DeviceCommandStatusSent, ""); err != nil {
		t.Fatalf("UpdateDeviceCommandStatus(sent) failed: %v", err)
	}
	if err := store.UpdateDeviceCommandStatus(commandID, inter.DeviceCommandStatusAcked, ""); err != nil {
		t.Fatalf("UpdateDeviceCommandStatus(acked) failed: %v", err)
	}

	var status string
	var executedAt sql.NullTime
	err = sqlStore.db.QueryRow(`
		SELECT status, executed_at
		FROM integration_external_commands
		WHERE id = ? AND source = ?
	`, commandID, "goster_device").Scan(&status, &executedAt)
	if err != nil {
		t.Fatalf("query command row failed: %v", err)
	}
	if status != string(inter.DeviceCommandStatusAcked) {
		t.Fatalf("unexpected command status: %s", status)
	}
	if !executedAt.Valid {
		t.Fatalf("acked command should have executed_at")
	}
}
