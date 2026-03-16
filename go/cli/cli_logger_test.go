package cli

import (
	"testing"

	"github.com/nhirsama/Goster-IoT/src/logger"
)

func TestInitRootLogger(t *testing.T) {
	old := logger.Default()
	t.Cleanup(func() {
		logger.SetDefault(old)
	})

	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_SERVICE", "goster")
	t.Setenv("LOG_ENV", "test")

	got := initRootLogger(logger.Config{
		Level:     "debug",
		Format:    "json",
		AddSource: false,
		Service:   "goster",
		Env:       "test",
	})
	if got == nil {
		t.Fatal("initRootLogger should return logger")
	}
	if logger.Default() == nil {
		t.Fatal("default logger should be set")
	}
}
