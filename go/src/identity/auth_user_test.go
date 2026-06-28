package identity

import (
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestAuthUserAccessors(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	var user AuthUser

	user.PutPID("pid")
	user.PutUsername("user")
	user.PutPassword("hashed")
	user.PutEmail("user@test.local")
	user.PutConfirmed(true)
	user.PutConfirmSelector("confirm-token")
	user.PutRecoverSelector("recover-token")
	user.PutRecoverExpiry(now)
	user.PutOAuth2UID("uid-1")
	user.PutOAuth2Provider("github")
	user.PutOAuth2AccessToken("access")
	user.PutOAuth2RefreshToken("refresh")
	user.PutOAuth2Expiry(now)
	user.PutRememberToken("remember")
	user.Permission = int(inter.PermissionReadWrite)

	if user.GetPID() != "user" {
		t.Fatalf("unexpected pid: %s", user.GetPID())
	}
	if user.GetUsername() != "user" {
		t.Fatalf("unexpected username: %s", user.GetUsername())
	}
	if user.GetPassword() != "hashed" {
		t.Fatalf("unexpected password: %s", user.GetPassword())
	}
	if user.GetEmail() != "user@test.local" {
		t.Fatalf("unexpected email: %s", user.GetEmail())
	}
	if !user.GetConfirmed() {
		t.Fatal("expected confirmed user")
	}
	if user.GetConfirmSelector() != "confirm-token" {
		t.Fatalf("unexpected confirm token: %s", user.GetConfirmSelector())
	}
	if user.GetRecoverSelector() != "recover-token" {
		t.Fatalf("unexpected recover token: %s", user.GetRecoverSelector())
	}
	if !user.GetRecoverExpiry().Equal(now) {
		t.Fatalf("unexpected recover expiry: %v", user.GetRecoverExpiry())
	}
	if user.GetOAuth2UID() != "uid-1" || user.GetOAuth2Provider() != "github" {
		t.Fatalf("unexpected oauth identity: uid=%s provider=%s", user.GetOAuth2UID(), user.GetOAuth2Provider())
	}
	if user.GetOAuth2AccessToken() != "access" || user.GetOAuth2RefreshToken() != "refresh" {
		t.Fatal("unexpected oauth tokens")
	}
	if !user.GetOAuth2Expiry().Equal(now) {
		t.Fatalf("unexpected oauth expiry: %v", user.GetOAuth2Expiry())
	}
	if user.GetRememberToken() != "remember" {
		t.Fatalf("unexpected remember token: %s", user.GetRememberToken())
	}
	if user.GetPermission() != inter.PermissionReadWrite {
		t.Fatalf("unexpected permission: %v", user.GetPermission())
	}
	if !user.IsOAuth2User() {
		t.Fatal("expected oauth user")
	}
}
