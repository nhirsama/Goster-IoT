package device_manager

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestTelemetryIngestServiceWritesMetricsAndLogs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "telemetry.db")
	ds, err := datastore.NewDataStoreSql(dbPath)
	if err != nil {
		t.Fatalf("failed to init datastore: %v", err)
	}

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
