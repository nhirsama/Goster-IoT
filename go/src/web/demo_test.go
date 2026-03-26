package web

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/core"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/persistence"
)

type heartbeatDeadlineSetter interface {
	SetHeartbeatDeadline(deadline time.Duration)
}

// TestRunServerAndStressTest sets up the server, populates data, and runs a stress test.
// Run with: go test -v ./go/src/web -run TestRunServerAndStressTest
func TestRunServerAndStressTest(t *testing.T) {
	probe, err := net.Listen("tcp", ":8080")
	if err != nil {
		t.Skipf("tcp listener on :8080 is unavailable in current environment: %v", err)
	}
	_ = probe.Close()

	// 1. 打印当前测试工作目录，便于定位调试信息。
	wd, _ := os.Getwd()
	fmt.Println("Starting Test in:", wd)

	// 2. Setup Temporary datastore
	tempDir, err := os.MkdirTemp("", "goster_test_db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir) // Clean up after test
	dbPath := filepath.Join(tempDir, "test_data.db")

	ds, err := persistence.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("Failed to open datastore at %s: %v", dbPath, err)
	}
	fmt.Printf("Using temporary database: %s\n", dbPath)

	// 3. Setup Managers
	services := core.NewServicesWithConfig(ds, appcfg.DeviceManagerConfig{
		QueueCapacity:     128,
		HeartbeatDeadline: 60 * time.Second,
	})

	// 测试里缩短在线判定窗口，便于更快覆盖在线/离线状态切换。
	if impl, ok := services.DevicePresence.(heartbeatDeadlineSetter); ok {
		impl.SetHeartbeatDeadline(5 * time.Second)
	}

	// Setup Authboss
	ab, err := SetupAuthboss(ds)
	if err != nil {
		t.Fatal(err)
	}
	authService, err := NewAuthService(ab)
	if err != nil {
		t.Fatal(err)
	}

	// 4. Create WebServer
	ws, err := NewWebServer(WebServerDeps{
		DataStore:        ds,
		DeviceRegistry:   services.DeviceRegistry,
		DevicePresence:   services.DevicePresence,
		DownlinkCommands: services.DownlinkCommands,
		Auth:             authService,
		Captcha:          &TurnstileService{Enabled: false},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 5. Populate Data
	populateData(t, ds, services.DevicePresence)

	// 6. Start Server in Goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if err := ws.Start(ctx); err != nil {
			t.Errorf("web server exited with error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(1 * time.Second)

	fmt.Println("------------------------------------------------")
	fmt.Println("web Server running on :8080 backed by Temp DB")
	fmt.Println("Starting Stress Test...")
	fmt.Println("------------------------------------------------")

	// 7. Run Stress Test
	runStressTest(t)

	// Uncomment to keep server running for manual inspection
	// select {}
}

func populateData(t *testing.T, ds inter.DataStore, presence inter.DevicePresence) {
	// Users
	// ds.RegisterUser("admin", "admin123", inter.PermissionAdmin)
	// ds.RegisterUser("viewer", "view123", inter.PermissionReadOnly)

	// Devices
	// 50 Online
	for i := 0; i < 500; i++ {
		uuid := fmt.Sprintf("dev-online-%03d", i)
		createDevice(ds, uuid, fmt.Sprintf("Sensor Online %d", i), inter.Authenticated)
		generateMetrics(ds, uuid, 100)
		presence.HandleHeartbeat(uuid) // Mark active
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
			"http://localhost:8080/api/v1/auth/captcha/config",
			"http://localhost:8080/api/v1/devices?status=all&page=1&size=50",
			"http://localhost:8080/api/v1/users",
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
