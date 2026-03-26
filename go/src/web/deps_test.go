package web

import (
	"testing"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
)

type testWebDepsDataStore struct {
	inter.DataStore
}

type testWebDepsRegistry struct {
	inter.DeviceRegistry
}

type testWebDepsPresence struct {
	inter.DevicePresence
}

type testWebDepsDownlinkCommands struct {
	inter.DownlinkCommandService
}

type testWebDepsAuth struct {
	AuthService
}

func TestWebServerDepsNormalizeLogger(t *testing.T) {
	deps := WebServerDeps{
		DataStore:        testWebDepsDataStore{},
		DeviceRegistry:   testWebDepsRegistry{},
		DevicePresence:   testWebDepsPresence{},
		DownlinkCommands: testWebDepsDownlinkCommands{},
		Auth:             testWebDepsAuth{},
		Captcha:          &TurnstileService{Enabled: false},
		Logger:           nil,
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

func TestWebServerLogFallback(t *testing.T) {
	ws := &webServer{}
	if ws.log() == nil {
		t.Fatal("web server logger fallback should not be nil")
	}
}

func TestWebServerDepsNormalizeRequiresDataStore(t *testing.T) {
	deps := WebServerDeps{}
	if err := deps.normalize(); err == nil {
		t.Fatal("expected missing runtime store error")
	}
}

func TestWebServerDepsNormalizeInjectsCaptchaAndConfig(t *testing.T) {
	deps := WebServerDeps{
		DataStore:        testWebDepsDataStore{},
		DeviceRegistry:   testWebDepsRegistry{},
		DevicePresence:   testWebDepsPresence{},
		DownlinkCommands: testWebDepsDownlinkCommands{},
		Auth:             testWebDepsAuth{},
		Logger:           logger.NewNoop(),
	}
	if err := deps.normalize(); err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if deps.Captcha == nil {
		t.Fatal("normalize should inject captcha service")
	}
	if deps.Config.HTTPAddr != appcfg.DefaultWebConfig().HTTPAddr {
		t.Fatalf("normalize should inject default web config, got %s", deps.Config.HTTPAddr)
	}
}
