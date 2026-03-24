package v1_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/inter"
	webpkg "github.com/nhirsama/Goster-IoT/src/web"
	apiv1 "github.com/nhirsama/Goster-IoT/src/web/v1"
)

func TestAPICaptchaConfigHandler(t *testing.T) {
	env := newTestAPI(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/captcha/config", nil)
	env.api.CaptchaConfigHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	envBody := mustJSONEnvelope(t, rec)
	data := envBody.Data.(map[string]interface{})
	if data["provider"] != "none" {
		t.Fatalf("unexpected provider: %v", data["provider"])
	}

	env = newTestAPI(t, apiTestOptions{
		captcha: &webpkg.TurnstileService{Enabled: true, SiteKey: "site_key_x"},
	})
	rec = httptest.NewRecorder()
	env.api.CaptchaConfigHandler(rec, req)
	envBody = mustJSONEnvelope(t, rec)
	data = envBody.Data.(map[string]interface{})
	if data["provider"] != "turnstile" || data["site_key"] != "site_key_x" {
		t.Fatalf("unexpected turnstile config: %+v", data)
	}
}

func TestAPIAuthHandlersValidation(t *testing.T) {
	env := newTestAPI(t)

	registerCases := []struct {
		body     string
		wantCode int
	}{
		{`{`, 40001},
		{`{"username":"ab","password":"12345678"}`, 40002},
		{`{"username":"abc","password":"123"}`, 40003},
		{`{"username":"abcd","password":"12345678","extra":"different"}`, 40001},
	}
	for _, tc := range registerCases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(tc.body))
		env.api.RegisterHandler(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("register expected 400, got %d", rec.Code)
		}
		if got := mustJSONEnvelope(t, rec).Code; got != tc.wantCode {
			t.Fatalf("register unexpected code: got %d want %d", got, tc.wantCode)
		}
	}

	env = newTestAPI(t, apiTestOptions{
		captcha: &webpkg.TurnstileService{Enabled: true},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		bytes.NewBufferString(`{"username":"validuser","password":"Admin123!"}`))
	env.api.RegisterHandler(rec, req)
	if got := mustJSONEnvelope(t, rec).Code; got != 40005 {
		t.Fatalf("expected captcha required 40005, got %d", got)
	}

	loginCases := []struct {
		body     string
		wantCode int
		wantHTTP int
	}{
		{`{`, 40007, http.StatusBadRequest},
		{`{"password":"12345678"}`, 40008, http.StatusBadRequest},
		{`{"username":"abc"}`, 40008, http.StatusBadRequest},
	}
	for _, tc := range loginCases {
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(tc.body))
		env.api.LoginHandler(rec, req)
		if rec.Code != tc.wantHTTP {
			t.Fatalf("login expected %d, got %d", tc.wantHTTP, rec.Code)
		}
		if got := mustJSONEnvelope(t, rec).Code; got != tc.wantCode {
			t.Fatalf("login unexpected code: got %d want %d", got, tc.wantCode)
		}
	}
}

func TestAPILogoutAndMeAndAuthMiddleware(t *testing.T) {
	env := newTestAPI(t)
	user := &datastore.AuthUser{
		Username:   "admin",
		Email:      "admin@test.local",
		Permission: int(inter.PermissionAdmin),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	env.api.LogoutHandler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("logout expected 401 without session, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req = req.WithContext(context.WithValue(req.Context(), authboss.CTXKeyUser, user))
	req = req.WithContext(context.WithValue(req.Context(), apiv1.ContextUsername, "admin"))
	req = req.WithContext(context.WithValue(req.Context(), apiv1.ContextPerm, inter.PermissionAdmin))
	rec = httptest.NewRecorder()
	env.api.MeHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("me expected 200, got %d", rec.Code)
	}
	meEnv := mustJSONEnvelope(t, rec)
	meData := meEnv.Data.(map[string]interface{})
	if meData["username"] != "admin" || meData["email"] != "admin@test.local" {
		t.Fatalf("unexpected me data: %+v", meData)
	}

	unauth := env.api.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next should not run for unauthorized request")
	}, inter.PermissionReadOnly)
	rec = httptest.NewRecorder()
	unauth.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil))
	if mustJSONEnvelope(t, rec).Code != 40101 {
		t.Fatalf("expected 40101 for unauthorized")
	}

	forbiddenReq := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	forbiddenReq = forbiddenReq.WithContext(context.WithValue(forbiddenReq.Context(), authboss.CTXKeyUser, &datastore.AuthUser{
		Username:   "viewer",
		Permission: int(inter.PermissionReadOnly),
	}))
	rec = httptest.NewRecorder()
	env.api.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next should not run for forbidden request")
	}, inter.PermissionAdmin).ServeHTTP(rec, forbiddenReq)
	if mustJSONEnvelope(t, rec).Code != 40301 {
		t.Fatalf("expected 40301 for forbidden")
	}

	nextCalled := false
	okReq := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	okReq = okReq.WithContext(context.WithValue(okReq.Context(), authboss.CTXKeyUser, user))
	rec = httptest.NewRecorder()
	env.api.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		if got := r.Context().Value(apiv1.ContextUsername); got != "admin" {
			t.Fatalf("unexpected context username: %v", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}, inter.PermissionReadOnly).ServeHTTP(rec, okReq)
	if !nextCalled || rec.Code != http.StatusNoContent {
		t.Fatalf("expected middleware to pass request")
	}
}

func TestAPIDeviceAndMetricsAndUsersHandlers(t *testing.T) {
	env := newTestAPI(t)

	uuid := strings.Repeat("a", 64)
	seedDevice(t, env.dataStore, uuid, inter.Authenticated)
	if err := env.dataStore.AppendMetric(uuid, inter.MetricPoint{
		Timestamp: time.Now().UnixMilli(),
		Value:     26.5,
		Type:      1,
	}); err != nil {
		t.Fatalf("seed metric failed: %v", err)
	}
	seedUser(t, env.dataStore, "admin_seed")
	seedUser(t, env.dataStore, "tester")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices?status=all&page=1&size=10", nil)
	env.api.DevicesHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("devices list expected 200, got %d", rec.Code)
	}
	if code := mustJSONEnvelope(t, rec).Code; code != 0 {
		t.Fatalf("devices list expected code 0, got %d", code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/devices/"+uuid, nil)
	rec = httptest.NewRecorder()
	env.api.DeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("device detail expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/devices/"+uuid+"/approve", nil)
	req = withPerm(req, inter.PermissionReadWrite)
	rec = httptest.NewRecorder()
	env.api.DeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/devices/"+uuid+"/token/refresh", nil)
	req = withPerm(req, inter.PermissionReadWrite)
	rec = httptest.NewRecorder()
	env.api.DeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh token expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/devices/"+uuid+"/commands",
		bytes.NewBufferString(`{"command":"action_exec","payload":{"op":"reboot"}}`))
	req = withPerm(req, inter.PermissionReadWrite)
	rec = httptest.NewRecorder()
	env.api.DeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("enqueue command expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	envBody := mustJSONEnvelope(t, rec)
	data, ok := envBody.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected enqueue response data: %#v", envBody.Data)
	}
	if data["status"] != string(inter.DeviceCommandStatusQueued) {
		t.Fatalf("unexpected command status: %v", data["status"])
	}

	dmsg, ok, err := env.downlinkCommands.PopDownlink(uuid)
	if err != nil {
		t.Fatalf("pop downlink failed: %v", err)
	}
	if !ok {
		t.Fatal("queued command should be available in device queue")
	}
	if dmsg.CmdID != inter.CmdActionExec || dmsg.CommandID <= 0 {
		t.Fatalf("unexpected queued message: %+v", dmsg)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/metrics/"+uuid+"?range=all", nil)
	rec = httptest.NewRecorder()
	env.api.MetricsHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	rec = httptest.NewRecorder()
	env.api.UsersHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("users expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/tester/permission", bytes.NewBufferString(`{"permission":1}`))
	rec = httptest.NewRecorder()
	env.api.UserPermissionHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update permission expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/devices/"+uuid, nil)
	req = withPerm(req, inter.PermissionReadWrite)
	rec = httptest.NewRecorder()
	env.api.DeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete expected 204, got %d", rec.Code)
	}
}

func TestAPIDeviceCommandValidation(t *testing.T) {
	env := newTestAPI(t)
	uuid := strings.Repeat("e", 64)
	seedDevice(t, env.dataStore, uuid, inter.Authenticated)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/"+uuid+"/commands",
		bytes.NewBufferString(`{"command":"reboot","payload":{"op":"now"}}`))
	req = withPerm(req, inter.PermissionReadWrite)
	rec := httptest.NewRecorder()
	env.api.DeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid command should return 400, got %d", rec.Code)
	}
	if code := mustJSONEnvelope(t, rec).Code; code != 40027 {
		t.Fatalf("invalid command should return code 40027, got %d", code)
	}
}

func TestAPIAuthRegisterLoginLogoutFlow(t *testing.T) {
	env := newTestAPI(t, apiTestOptions{
		captcha: &webpkg.TurnstileService{Enabled: false},
	})
	mux := newTestMux(env.api)

	registerRec := httptest.NewRecorder()
	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		bytes.NewBufferString(`{"username":"admin","password":"Admin123!","email":"admin@test.local"}`))
	mux.ServeHTTP(registerRec, registerReq)

	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register expected 201, got %d, body=%s", registerRec.Code, registerRec.Body.String())
	}
	if code := mustJSONEnvelope(t, registerRec).Code; code != 0 {
		t.Fatalf("register business code expected 0, got %d", code)
	}

	conflictRec := httptest.NewRecorder()
	conflictReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		bytes.NewBufferString(`{"username":"admin","password":"Admin123!"}`))
	mux.ServeHTTP(conflictRec, conflictReq)
	if conflictRec.Code != http.StatusConflict {
		t.Fatalf("duplicate register expected 409, got %d", conflictRec.Code)
	}
	if code := mustJSONEnvelope(t, conflictRec).Code; code != 40901 {
		t.Fatalf("duplicate register expected code 40901, got %d", code)
	}

	loginRec := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(`{"username":"admin","password":"Admin123!","remember_me":false}`))
	mux.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login expected 200, got %d, body=%s", loginRec.Code, loginRec.Body.String())
	}
	loginEnv := mustJSONEnvelope(t, loginRec)
	if loginEnv.Code != 0 {
		t.Fatalf("login business code expected 0, got %d", loginEnv.Code)
	}

	badLoginRec := httptest.NewRecorder()
	badLoginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(`{"username":"admin","password":"wrong-password","remember_me":false}`))
	mux.ServeHTTP(badLoginRec, badLoginReq)
	if badLoginRec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login expected 401, got %d", badLoginRec.Code)
	}
	if code := mustJSONEnvelope(t, badLoginRec).Code; code != 40112 {
		t.Fatalf("bad login expected code 40112, got %d", code)
	}

	logoutRec := httptest.NewRecorder()
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	for _, c := range loginRec.Result().Cookies() {
		logoutReq.AddCookie(c)
	}
	mux.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout expected 204, got %d", logoutRec.Code)
	}
}

func TestAPIDeviceDeleteNotFound(t *testing.T) {
	env := newTestAPI(t)
	uuid := strings.Repeat("b", 64)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/"+uuid, nil)
	req = withPerm(req, inter.PermissionReadWrite)
	rec := httptest.NewRecorder()

	env.api.DeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete not-found should return 404, got %d", rec.Code)
	}
}

func TestAPIRefreshTokenNotFound(t *testing.T) {
	env := newTestAPI(t)
	uuid := strings.Repeat("c", 64)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/"+uuid+"/token/refresh", nil)
	req = withPerm(req, inter.PermissionReadWrite)
	rec := httptest.NewRecorder()

	env.api.DeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("refresh token not-found should return 404, got %d", rec.Code)
	}
	if code := mustJSONEnvelope(t, rec).Code; code != 40425 {
		t.Fatalf("unexpected business code: %d", code)
	}
}

func TestAPIUserPermissionValidationAndNotFound(t *testing.T) {
	env := newTestAPI(t)
	seedUser(t, env.dataStore, "perm_user")
	beforePerm, err := env.dataStore.GetUserPermission("perm_user")
	if err != nil {
		t.Fatalf("failed to get initial permission: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/perm_user/permission",
		bytes.NewBufferString(`{"permission":"bad"}`))
	rec := httptest.NewRecorder()
	env.api.UserPermissionHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid permission should return 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if perm, err := env.dataStore.GetUserPermission("perm_user"); err != nil || perm != beforePerm {
		t.Fatalf("permission should remain unchanged after invalid request: perm=%d err=%v", perm, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/not_exist/permission",
		bytes.NewBufferString(`{"permission":2}`))
	rec = httptest.NewRecorder()
	env.api.UserPermissionHandler(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("user not found should return 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAPIDeviceTokenVisibilityByPermission(t *testing.T) {
	env := newTestAPI(t)
	uuid := strings.Repeat("d", 64)
	seedDevice(t, env.dataStore, uuid, inter.Authenticated)

	readonlyReq := httptest.NewRequest(http.MethodGet, "/api/v1/devices?status=all&page=1&size=10", nil)
	readonlyReq = withPerm(readonlyReq, inter.PermissionReadOnly)
	readonlyRec := httptest.NewRecorder()
	env.api.DevicesHandler(readonlyRec, readonlyReq)
	if readonlyRec.Code != http.StatusOK {
		t.Fatalf("readonly list expected 200, got %d", readonlyRec.Code)
	}
	readonlyEnv := mustJSONEnvelope(t, readonlyRec)
	readonlyData := readonlyEnv.Data.(map[string]interface{})
	items := readonlyData["items"].([]interface{})
	firstItem := items[0].(map[string]interface{})
	meta := firstItem["meta"].(map[string]interface{})
	if meta["token"] != nil {
		t.Fatalf("readonly user should not see token")
	}

	readwriteReq := httptest.NewRequest(http.MethodGet, "/api/v1/devices/"+uuid, nil)
	readwriteReq = withPerm(readwriteReq, inter.PermissionReadWrite)
	readwriteRec := httptest.NewRecorder()
	env.api.DeviceByUUIDHandler(readwriteRec, readwriteReq)
	if readwriteRec.Code != http.StatusOK {
		t.Fatalf("readwrite detail expected 200, got %d", readwriteRec.Code)
	}
	readwriteEnv := mustJSONEnvelope(t, readwriteRec)
	readwriteData := readwriteEnv.Data.(map[string]interface{})
	detailMeta := readwriteData["meta"].(map[string]interface{})
	if token, ok := detailMeta["token"].(string); !ok || strings.TrimSpace(token) == "" {
		t.Fatalf("readwrite user should see token")
	}
}

func TestAPIUserPermissionGuardsSelfAndLastAdmin(t *testing.T) {
	env := newTestAPI(t)
	seedUser(t, env.dataStore, "admin_only")

	selfReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/admin_only/permission",
		bytes.NewBufferString(`{"permission":1}`))
	selfReq = withUserPerm(selfReq, "admin_only", inter.PermissionAdmin)
	selfRec := httptest.NewRecorder()
	env.api.UserPermissionHandler(selfRec, selfReq)
	if selfRec.Code != http.StatusBadRequest {
		t.Fatalf("self demotion should return 400, got %d", selfRec.Code)
	}
	if code := mustJSONEnvelope(t, selfRec).Code; code != 40046 {
		t.Fatalf("self demotion expected code 40046, got %d", code)
	}

	lastAdminReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/admin_only/permission",
		bytes.NewBufferString(`{"permission":1}`))
	lastAdminReq = withUserPerm(lastAdminReq, "other_admin", inter.PermissionAdmin)
	lastAdminRec := httptest.NewRecorder()
	env.api.UserPermissionHandler(lastAdminRec, lastAdminReq)
	if lastAdminRec.Code != http.StatusBadRequest {
		t.Fatalf("last admin demotion should return 400, got %d", lastAdminRec.Code)
	}
	if code := mustJSONEnvelope(t, lastAdminRec).Code; code != 40047 {
		t.Fatalf("last admin demotion expected code 40047, got %d", code)
	}
}

func TestAPIDeviceHandlersRespectTenantScope(t *testing.T) {
	env := newTestAPI(t)
	uuid := strings.Repeat("f", 64)
	seedDevice(t, env.dataStore, uuid, inter.Authenticated)

	tenantOtherListReq := httptest.NewRequest(http.MethodGet, "/api/v1/devices?status=all&page=1&size=10", nil)
	tenantOtherListReq.Header.Set("X-Tenant-Id", "tenant_other")
	tenantOtherListReq = withPerm(tenantOtherListReq, inter.PermissionReadOnly)
	tenantOtherListRec := httptest.NewRecorder()
	env.api.DevicesHandler(tenantOtherListRec, tenantOtherListReq)
	if tenantOtherListRec.Code != http.StatusOK {
		t.Fatalf("tenant_other list expected 200, got %d", tenantOtherListRec.Code)
	}
	otherListEnv := mustJSONEnvelope(t, tenantOtherListRec)
	otherListData := otherListEnv.Data.(map[string]interface{})
	if got := len(otherListData["items"].([]interface{})); got != 0 {
		t.Fatalf("tenant_other should not see legacy device, got=%d", got)
	}

	legacyListReq := httptest.NewRequest(http.MethodGet, "/api/v1/devices?status=all&page=1&size=10", nil)
	legacyListReq.Header.Set("X-Tenant-Id", "tenant_legacy")
	legacyListReq = withPerm(legacyListReq, inter.PermissionReadOnly)
	legacyListRec := httptest.NewRecorder()
	env.api.DevicesHandler(legacyListRec, legacyListReq)
	if legacyListRec.Code != http.StatusOK {
		t.Fatalf("tenant_legacy list expected 200, got %d", legacyListRec.Code)
	}
	legacyListEnv := mustJSONEnvelope(t, legacyListRec)
	legacyListData := legacyListEnv.Data.(map[string]interface{})
	if got := len(legacyListData["items"].([]interface{})); got == 0 {
		t.Fatalf("tenant_legacy should see seeded device")
	}

	tenantOtherDetailReq := httptest.NewRequest(http.MethodGet, "/api/v1/devices/"+uuid, nil)
	tenantOtherDetailReq.Header.Set("X-Tenant-Id", "tenant_other")
	tenantOtherDetailReq = withPerm(tenantOtherDetailReq, inter.PermissionReadOnly)
	tenantOtherDetailRec := httptest.NewRecorder()
	env.api.DeviceByUUIDHandler(tenantOtherDetailRec, tenantOtherDetailReq)
	if tenantOtherDetailRec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant detail should return 404, got %d", tenantOtherDetailRec.Code)
	}
	if code := mustJSONEnvelope(t, tenantOtherDetailRec).Code; code != 40421 {
		t.Fatalf("cross-tenant detail code mismatch: got %d want 40421", code)
	}
}

func TestAPIMeIncludesActiveTenant(t *testing.T) {
	env := newTestAPI(t)
	user := &datastore.AuthUser{
		Username:   "tenant_user",
		Email:      "tenant_user@test.local",
		Permission: int(inter.PermissionReadOnly),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("X-Tenant-Id", "tenant_demo")
	req = req.WithContext(context.WithValue(req.Context(), authboss.CTXKeyUser, user))
	req = req.WithContext(context.WithValue(req.Context(), apiv1.ContextUsername, "tenant_user"))
	req = req.WithContext(context.WithValue(req.Context(), apiv1.ContextPerm, inter.PermissionReadOnly))
	rec := httptest.NewRecorder()

	env.api.MeHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("me expected 200, got %d", rec.Code)
	}
	envBody := mustJSONEnvelope(t, rec)
	data := envBody.Data.(map[string]interface{})
	if got := data["active_tenant"]; got != "tenant_demo" {
		t.Fatalf("active_tenant mismatch: got=%v want=tenant_demo", got)
	}
}
