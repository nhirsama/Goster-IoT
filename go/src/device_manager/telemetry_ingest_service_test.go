package device_manager

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/persistence"
)

func TestTelemetryIngestServiceWritesMetricsAndLogs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "telemetry.db")
	ds, err := persistence.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to init runtime store: %v", err)
	}
	t.Cleanup(func() {
		_ = persistence.CloseIfPossible(ds)
	})

	uuid := "device-telemetry"
	if err := ds.InitDevice(uuid, inter.DeviceMetadata{
		Name:               "Device Telemetry",
		SerialNumber:       "sn-telemetry",
		MACAddress:         "mac-telemetry",
		Token:              "tk-telemetry",
		AuthenticateStatus: inter.Authenticated,
	}); err != nil {
		t.Fatalf("failed to init device: %v", err)
	}

	service := NewTelemetryIngestService(ds)
	startTS := time.Now().Add(-time.Minute).UnixMilli()
	if err := service.IngestMetrics(uuid, []inter.MetricPoint{
		{Timestamp: startTS, Value: 21.5, Type: 1},
		{Timestamp: startTS + 1000, Value: 22.0, Type: 1},
	}); err != nil {
		t.Fatalf("ingest metrics failed: %v", err)
	}

	points, err := ds.QueryMetrics(uuid, startTS-1, startTS+2000)
	if err != nil {
		t.Fatalf("query metrics failed: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("unexpected metric count: got %d want 2", len(points))
	}

	logTS := time.Now().UnixMilli()
	if err := service.IngestLog(uuid, inter.LogUploadData{
		Timestamp: logTS,
		Level:     inter.LogLevelWarn,
		Message:   "sensor drift",
	}); err != nil {
		t.Fatalf("ingest log failed: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	var level string
	var message string
	if err := db.QueryRow("SELECT level, message FROM logs WHERE uuid = ? ORDER BY id DESC LIMIT 1", uuid).Scan(&level, &message); err != nil {
		t.Fatalf("failed to query log record: %v", err)
	}
	if level != "WARN" || !strings.Contains(message, "sensor drift") {
		t.Fatalf("unexpected log record: level=%s message=%s", level, message)
	}
}

func TestTelemetryIngestServiceWritesEventAndError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "telemetry_event.db")
	ds, err := persistence.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to init runtime store: %v", err)
	}
	t.Cleanup(func() {
		_ = persistence.CloseIfPossible(ds)
	})

	uuid := "device-telemetry-event"
	if err := ds.InitDevice(uuid, inter.DeviceMetadata{
		Name:               "Device Telemetry Event",
		SerialNumber:       "sn-telemetry-event",
		MACAddress:         "mac-telemetry-event",
		Token:              "tk-telemetry-event",
		AuthenticateStatus: inter.Authenticated,
	}); err != nil {
		t.Fatalf("failed to init device: %v", err)
	}

	service := NewTelemetryIngestService(ds)
	if err := service.IngestEvent(uuid, []byte(`{"event":"boot"}`)); err != nil {
		t.Fatalf("IngestEvent failed: %v", err)
	}
	if err := service.IngestDeviceError(uuid, []byte(`panic`)); err != nil {
		t.Fatalf("IngestDeviceError failed: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_loc=Local")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT level, message FROM logs WHERE uuid = ? ORDER BY id ASC", uuid)
	if err != nil {
		t.Fatalf("failed to query log records: %v", err)
	}
	defer rows.Close()

	var levels []string
	for rows.Next() {
		var level string
		var message string
		if err := rows.Scan(&level, &message); err != nil {
			t.Fatalf("scan logs failed: %v", err)
		}
		levels = append(levels, level+":"+message)
	}
	if len(levels) != 2 || !strings.HasPrefix(levels[0], "EVENT:") || !strings.HasPrefix(levels[1], "ERROR:") {
		t.Fatalf("unexpected event/error logs: %+v", levels)
	}
}
