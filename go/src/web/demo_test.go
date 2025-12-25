package web

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/DataStore"
	"github.com/nhirsama/Goster-IoT/src/DeviceManager"
	"github.com/nhirsama/Goster-IoT/src/IdentityManager"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

// TestRunServer sets up the full stack and runs the web server.
// Run this with: go test -v ./go/src/web -run TestRunServer
func TestRunServer(t *testing.T) {
	// 1. Change to project root so "go/html/..." paths work
	if err := os.Chdir("../../.."); err != nil {
		t.Fatal(err)
	}

	cwd, _ := os.Getwd()
	fmt.Println("Current Working Directory:", cwd)

	// 2. Setup DataStore (Use existing DB)
	dbPath := "go/src/DataStore/data.db"

	ds, err := DataStore.NewDataStoreSql(dbPath)
	if err != nil {
		t.Fatalf("Failed to open DataStore at %s: %v", dbPath, err)
	}

	// 3. Setup Managers
	im := IdentityManager.NewIdentityManager(ds)
	dm := DeviceManager.NewDeviceManager(ds, im)

	// Hack: Set DeathLine on DeviceManager implementation manually
	if impl, ok := dm.(*DeviceManager.DeviceManager); ok {
		impl.DeathLine = 30 * time.Second
		fmt.Println("DeviceManager DeathLine set to 30s")
	}

	// 4. Create WebServer
	ws := NewWebServer(ds, dm)

	// 5. Populate Dummy Data (Ignore errors if devices exist)
	// Device 1: Online
	uuid1 := "device-online-001"
	createDevice(t, ds, uuid1, "Temperature Sensor A", inter.Authenticated)
	generateMetrics(t, ds, uuid1)

	// Device 2: Offline
	uuid2 := "device-offline-002"
	createDevice(t, ds, uuid2, "Humidity Sensor B", inter.Authenticated)

	// Device 3: Pending
	uuid3 := "device-pending-003"
	createDevice(t, ds, uuid3, "Smart Light (New)", inter.AuthenticatePending)

	// Background heartbeat loop
	go func() {
		// Initial heartbeat
		dm.HandleHeartbeat(uuid1)
		dm.HandleHeartbeat(uuid3)

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			dm.HandleHeartbeat(uuid1)
			dm.HandleHeartbeat(uuid3)
			<-ticker.C
		}
	}()

	// 6. Start Server
	go ws.Start()

	fmt.Println("------------------------------------------------")
	fmt.Println("Web Server is running at http://localhost:8080")
	fmt.Println("Using Database: ", dbPath)
	fmt.Println("You can access the dashboard to see the devices.")
	fmt.Println("Press Ctrl+C to stop this test.")
	fmt.Println("------------------------------------------------")

	// Block forever
	select {}
}

func createDevice(t *testing.T, ds inter.DataStore, uuid, name string, status inter.AuthenticateStatusType) {
	err := ds.InitDevice(uuid, inter.DeviceMetadata{
		Name:               name,
		HWVersion:          "v1.0",
		SWVersion:          "v1.0",
		SerialNumber:       "SN-" + uuid,
		MACAddress:         "AA:BB:CC:DD:EE:FF",
		CreatedAt:          time.Now(),
		Token:              "token-" + uuid,
		AuthenticateStatus: status,
	})
	if err != nil {
		// Ignore error if device likely exists
	}
}

func generateMetrics(t *testing.T, ds inter.DataStore, uuid string) {
	now := time.Now().Unix()
	// Generate data for the last 7 days (approx 1000 points, 1 per 10 mins)
	for i := 0; i < 1008; i++ {
		ds.AppendMetric(uuid, inter.MetricPoint{
			Timestamp: now - int64(1008-i)*600, // Every 10 minutes
			Value:     20.0 + rand.Float32()*10.0,
		})
	}
}
