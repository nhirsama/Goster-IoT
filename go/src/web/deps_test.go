package web

import (
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
)

type testWebDepsDataStore struct {
	inter.DataStore
}

type testWebDepsDeviceManager struct {
	inter.DeviceManager
}

type testWebDepsAPI struct {
	inter.Api
}

type testWebDepsAuth struct {
	AuthService
}

func TestWebServerDepsNormalizeLogger(t *testing.T) {
	deps := WebServerDeps{
		DataStore:     testWebDepsDataStore{},
		DeviceManager: testWebDepsDeviceManager{},
		API:           testWebDepsAPI{},
		Auth:          testWebDepsAuth{},
		Captcha:       &TurnstileService{Enabled: false},
		Logger:        nil,
	}

	old := logger.Default()
	t.Cleanup(func() { logger.SetDefault(old) })
	logger.SetDefault(logger.NewNoop())

	if err := deps.normalize(); err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if deps.Logger == nil {
		t.Fatal("normalize should inject default logger")
	}
}
