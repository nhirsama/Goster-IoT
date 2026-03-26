package web

import (
	"testing"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/persistence"
)

func TestSetupAuthbossWithConfigRejectsNonIdentityStore(t *testing.T) {
	_, err := SetupAuthbossWithConfig(testWebDepsDataStore{}, appcfg.DefaultAuthConfig())
	if err == nil {
		t.Fatal("expected non-identity store to be rejected")
	}
}

func TestSetupAuthbossWithConfigSupportsLegacyStore(t *testing.T) {
	ds, err := persistence.OpenSQLite(t.TempDir() + "/auth_setup.db")
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() {
		_ = persistence.CloseIfPossible(ds)
	})

	ab, err := SetupAuthbossWithConfig(ds, appcfg.DefaultAuthConfig())
	if err != nil {
		t.Fatalf("SetupAuthbossWithConfig failed: %v", err)
	}
	if ab == nil {
		t.Fatal("expected authboss instance")
	}
}
