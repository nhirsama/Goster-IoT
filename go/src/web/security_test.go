package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestProtectedRoutesRejectUnauthorizedTenantSwitch(t *testing.T) {
	ws := newAuthFlowWS(t)
	mux := http.NewServeMux()
	ws.registerAPIRoutes(mux)

	seedDevice(t, ws.dataStore, strings.Repeat("a", 64), inter.Authenticated)

	registerAPITestUser(t, mux, "admin", "Admin123!")
	registerAPITestUser(t, mux, "viewer", "Viewer123!")
	if err := ws.dataStore.UpdateUserPermission("viewer", inter.PermissionReadOnly); err != nil {
		t.Fatalf("failed to grant viewer read access: %v", err)
	}

	viewerCookies := loginAPITestUser(t, mux, "viewer", "Viewer123!", "198.51.100.10:1234")

	forbiddenReq := httptest.NewRequest(http.MethodGet, "/api/v1/devices?status=all&page=1&size=10", nil)
	forbiddenReq.Header.Set("X-Tenant-Id", "tenant_other")
	for _, c := range viewerCookies {
		forbiddenReq.AddCookie(c)
	}
	forbiddenRec := httptest.NewRecorder()
	mux.ServeHTTP(forbiddenRec, forbiddenReq)
	if forbiddenRec.Code != http.StatusForbidden {
		t.Fatalf("cross-tenant list should return 403, got %d body=%s", forbiddenRec.Code, forbiddenRec.Body.String())
	}
	if code := mustJSONEnvelope(t, forbiddenRec).Code; code != 40303 {
		t.Fatalf("cross-tenant list expected code 40303, got %d", code)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.Header.Set("X-Tenant-Id", "tenant_other")
	for _, c := range viewerCookies {
		meReq.AddCookie(c)
	}
	meRec := httptest.NewRecorder()
	mux.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusForbidden {
		t.Fatalf("cross-tenant me should return 403, got %d body=%s", meRec.Code, meRec.Body.String())
	}

	allowedReq := httptest.NewRequest(http.MethodGet, "/api/v1/devices?status=all&page=1&size=10", nil)
	allowedReq.Header.Set("X-Tenant-Id", "tenant_legacy")
	for _, c := range viewerCookies {
		allowedReq.AddCookie(c)
	}
	allowedRec := httptest.NewRecorder()
	mux.ServeHTTP(allowedRec, allowedReq)
	if allowedRec.Code != http.StatusOK {
		t.Fatalf("legacy tenant list should return 200, got %d body=%s", allowedRec.Code, allowedRec.Body.String())
	}
}

func TestLoginGuardLocksRepeatedFailures(t *testing.T) {
	ws := newAuthFlowWS(t)
	ws.loginGuard = newLoginAttemptGuard(appcfg.LoginProtectionConfig{
		MaxFailures: 2,
		Window:      time.Minute,
		Lockout:     time.Minute,
	})
	now := time.Unix(1_710_000_000, 0).UTC()
	ws.loginGuard.SetClockForTest(func() time.Time { return now })

	mux := http.NewServeMux()
	ws.registerAPIRoutes(mux)

	registerAPITestUser(t, mux, "admin", "Admin123!")

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
			bytes.NewBufferString(`{"username":"admin","password":"wrong-password","remember_me":false}`))
		req.RemoteAddr = "203.0.113.9:4321"
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("wrong password attempt %d should return 401, got %d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}

	blockedRec := httptest.NewRecorder()
	blockedReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(`{"username":"admin","password":"Admin123!","remember_me":false}`))
	blockedReq.RemoteAddr = "203.0.113.9:4321"
	mux.ServeHTTP(blockedRec, blockedReq)
	if blockedRec.Code != http.StatusTooManyRequests {
		t.Fatalf("locked login should return 429, got %d body=%s", blockedRec.Code, blockedRec.Body.String())
	}
	if code := mustJSONEnvelope(t, blockedRec).Code; code != 42911 {
		t.Fatalf("locked login expected code 42911, got %d", code)
	}
	if blockedRec.Header().Get("Retry-After") == "" {
		t.Fatalf("locked login should include Retry-After header")
	}

	now = now.Add(61 * time.Second)
	unlockedRec := httptest.NewRecorder()
	unlockedReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(`{"username":"admin","password":"Admin123!","remember_me":false}`))
	unlockedReq.RemoteAddr = "203.0.113.9:4321"
	mux.ServeHTTP(unlockedRec, unlockedReq)
	if unlockedRec.Code != http.StatusOK {
		t.Fatalf("login after lockout should return 200, got %d body=%s", unlockedRec.Code, unlockedRec.Body.String())
	}

	postSuccessRec := httptest.NewRecorder()
	postSuccessReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(`{"username":"admin","password":"wrong-password","remember_me":false}`))
	postSuccessReq.RemoteAddr = "203.0.113.9:4321"
	mux.ServeHTTP(postSuccessRec, postSuccessReq)
	if postSuccessRec.Code != http.StatusUnauthorized {
		t.Fatalf("single failure after successful login should not stay locked, got %d body=%s", postSuccessRec.Code, postSuccessRec.Body.String())
	}
}

func registerAPITestUser(t *testing.T, mux *http.ServeMux, username, password string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		bytes.NewBufferString(`{"username":"`+username+`","password":"`+password+`","email":"`+username+`@test.local"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register %s failed: status=%d body=%s", username, rec.Code, rec.Body.String())
	}
}

func loginAPITestUser(t *testing.T, mux *http.ServeMux, username, password, remoteAddr string) []*http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(`{"username":"`+username+`","password":"`+password+`","remember_me":false}`))
	req.RemoteAddr = remoteAddr
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s failed: status=%d body=%s", username, rec.Code, rec.Body.String())
	}
	return rec.Result().Cookies()
}
