package Web

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/DataStore"
	"github.com/nhirsama/Goster-IoT/src/DeviceManager"
	"github.com/nhirsama/Goster-IoT/src/IdentityManager"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

// MockApi implements inter.Api for testing purposes
type MockApi struct{}

func (m *MockApi) Start()                                                        {}
func (m *MockApi) Handshake(uuid, token string) (string, error)                  { return "", nil }
func (m *MockApi) Heartbeat(uuid string) (bool, error)                           { return false, nil }
func (m *MockApi) UploadMetrics(uuid string, data inter.MetricsUploadData) error { return nil }
func (m *MockApi) UploadLog(uuid, level, message string) error                   { return nil }
func (m *MockApi) GetMessages(uuid string) ([]interface{}, error)                { return nil, nil }

// TestRunServerAndStressTest sets up the server, populates data, and runs a stress test.
// Run with: go test -v ./go/src/Web -run TestRunServerAndStressTest
func TestRunServerAndStressTest(t *testing.T) {
	// 1. Change to project root so "go/html/..." paths work
	// Adjust this depending on where you run the test from.
	// Assuming running from project root or having robust path handling.
	// For this test, we try to find the project root.
	wd, _ := os.Getwd()
	fmt.Println("Starting Test in:", wd)

	// Try to locate 'go/html'
	htmlDir := "../../html" // Relative from go/src/Web
	if _, err := os.Stat(htmlDir); os.IsNotExist(err) {
		// Try absolute path if known, or fail
		t.Logf("Warning: Could not find html dir at %s, trying absolute path logic or skipping template loading checks if flexible.", htmlDir)
	}

	// 2. Setup Temporary DataStore
	tempDir, err := os.MkdirTemp("", "goster_test_db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir) // Clean up after test
	dbPath := filepath.Join(tempDir, "test_data.db")

	ds, err := DataStore.NewDataStoreSql(dbPath)
	if err != nil {
		t.Fatalf("Failed to open DataStore at %s: %v", dbPath, err)
	}
	fmt.Printf("Using temporary database: %s\n", dbPath)

	// 3. Setup Managers
	im := IdentityManager.NewIdentityManager(ds)
	dm := DeviceManager.NewDeviceManager(ds, im)
	api := &MockApi{}

	// Hack: Set DeathLine on DeviceManager implementation manually
	if impl, ok := dm.(*DeviceManager.DeviceManager); ok {
		impl.DeathLine = 5 * time.Second // Shorten for test
	}

	// 4. Create WebServer
	ws := NewWebServer(ds, dm, api, htmlDir)

	// 5. Populate Data
	populateData(t, ds, dm)

	// 6. Start Server in Goroutine
	go ws.Start()

	// Wait for server to start
	time.Sleep(1 * time.Second)

	fmt.Println("------------------------------------------------")
	fmt.Println("Web Server running on :8080 backed by Temp DB")
	fmt.Println("Starting Stress Test...")
	fmt.Println("------------------------------------------------")

	// 7. Run Stress Test
	runStressTest(t)

	// Uncomment to keep server running for manual inspection
	// select {}
}

func populateData(t *testing.T, ds inter.DataStore, dm inter.DeviceManager) {
	// Users
	ds.RegisterUser("admin", "admin123", inter.PermissionAdmin)
	ds.RegisterUser("viewer", "view123", inter.PermissionReadOnly)

	// Devices
	// 50 Online
	for i := 0; i < 500; i++ {
		uuid := fmt.Sprintf("dev-online-%03d", i)
		createDevice(ds, uuid, fmt.Sprintf("Sensor Online %d", i), inter.Authenticated)
		generateMetrics(ds, uuid, 100)
		dm.HandleHeartbeat(uuid) // Mark active
	}

	// 50 Offline
	for i := 0; i < 500; i++ {
		uuid := fmt.Sprintf("dev-offline-%03d", i)
		createDevice(ds, uuid, fmt.Sprintf("Sensor Offline %d", i), inter.Authenticated)
		// No heartbeat -> Offline
	}

	// 20 Pending
	for i := 0; i < 200; i++ {
		uuid := fmt.Sprintf("dev-pending-%03d", i)
		createDevice(ds, uuid, fmt.Sprintf("New Device %d", i), inter.AuthenticatePending)
	}

	// 10 Blacklisted
	for i := 0; i < 100; i++ {
		uuid := fmt.Sprintf("dev-block-%03d", i)
		createDevice(ds, uuid, fmt.Sprintf("Bad Device %d", i), inter.AuthenticateRefuse)
	}
}

func createDevice(ds inter.DataStore, uuid, name string, status inter.AuthenticateStatusType) {
	ds.InitDevice(uuid, inter.DeviceMetadata{
		Name:               name,
		HWVersion:          "v1.0",
		SerialNumber:       "SN-" + uuid,
		CreatedAt:          time.Now(),
		Token:              "tk-" + uuid,
		AuthenticateStatus: status,
	})
}

func generateMetrics(ds inter.DataStore, uuid string, count int) {
	now := time.Now().Unix()
	var points []inter.MetricPoint
	for i := 0; i < count; i++ {
		points = append(points, inter.MetricPoint{
			Timestamp: now - int64(count-i)*60,
			Value:     20.0 + rand.Float32()*10.0,
		})
	}
	ds.BatchAppendMetrics(uuid, points)
}

func runStressTest(t *testing.T) {
	var wg sync.WaitGroup
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	start := time.Now()
	totalRequests := 100000
	concurrency := 5000

	requestCh := make(chan string, totalRequests)

	// Generate requests
	go func() {
		endpoints := []string{
			"http://localhost:8080/login",
			// Protected endpoints will redirect to login, still valid for load testing
			"http://localhost:8080/",
			"http://localhost:8080/devices",
		}
		for i := 0; i < totalRequests; i++ {
			requestCh <- endpoints[rand.Intn(len(endpoints))]
		}
		close(requestCh)
	}()

	// Workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range requestCh {
				resp, err := client.Get(url)
				if err != nil {
					// Connection errors are expected if server is overloaded or starting
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)
	rps := float64(totalRequests) / duration.Seconds()

	fmt.Printf("Stress Test Completed:\n")
	fmt.Printf("Total Requests: %d\n", totalRequests)
	fmt.Printf("Concurrency: %d\n", concurrency)
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("RPS: %.2f\n", rps)
}
