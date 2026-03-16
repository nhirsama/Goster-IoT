package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/device_manager"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

func newTestWS(t *testing.T) (*webServer, inter.DataStore, inter.DeviceManager) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	ds, err := datastore.NewDataStoreSql(dbPath)
	if err != nil {
		t.Fatalf("failed to init datastore: %v", err)
	}
	dm := device_manager.NewDeviceManager(ds)
	ab, err := SetupAuthboss(ds)
	if err != nil {
		t.Fatalf("failed to setup authboss: %v", err)
	}
	authService, err := NewAuthService(ab)
	if err != nil {
		t.Fatalf("failed to setup auth service: %v", err)
	}

	return &webServer{
		dataStore:     ds,
		deviceManager: dm,
		auth:          authService,
		captcha:       &TurnstileService{},
	}, ds, dm
}

func mustJSONEnvelope(t *testing.T, rec *httptest.ResponseRecorder) apiEnvelope {
	t.Helper()
	var env apiEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode envelope: %v, body=%s", err, rec.Body.String())
	}
	return env
}

func ctxWithPerm(req *http.Request, perm inter.PermissionType) *http.Request {
	ctx := context.WithValue(req.Context(), apiCtxPerm, perm)
	ctx = context.WithValue(ctx, apiCtxRequestID, "req_test")
	return req.WithContext(ctx)
}

func ctxWithUserPerm(req *http.Request, username string, perm inter.PermissionType) *http.Request {
	ctx := context.WithValue(req.Context(), apiCtxUsername, username)
	ctx = context.WithValue(ctx, apiCtxPerm, perm)
	ctx = context.WithValue(ctx, apiCtxRequestID, "req_test")
	return req.WithContext(ctx)
}

func seedDevice(t *testing.T, ds inter.DataStore, uuid string, status inter.AuthenticateStatusType) {
	t.Helper()
	meta := inter.DeviceMetadata{
		Name:               "Device-" + uuid,
		HWVersion:          "v1",
		SWVersion:          "v1",
		ConfigVersion:      "v1",
		SerialNumber:       "sn-" + uuid,
		MACAddress:         "mac-" + uuid,
		CreatedAt:          time.Now().UTC(),
		Token:              "tk-" + uuid,
		AuthenticateStatus: status,
	}
	if err := ds.InitDevice(uuid, meta); err != nil {
		t.Fatalf("failed to seed device: %v", err)
	}
}

func seedUser(t *testing.T, ds inter.DataStore, username string) {
	t.Helper()
	storer, ok := ds.(authboss.CreatingServerStorer)
	if !ok {
		t.Fatalf("datastore does not implement CreatingServerStorer")
	}
	u := &datastore.AuthUser{
		Username: username,
		Password: "plain_pw_for_tests",
	}
	if err := storer.Create(context.Background(), u); err != nil && err != authboss.ErrUserFound {
		t.Fatalf("failed to seed user: %v", err)
	}
}

func newAuthFlowWS(t *testing.T) *webServer {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "auth_flow.db")
	ds, err := datastore.NewDataStoreSql(dbPath)
	if err != nil {
		t.Fatalf("failed to init datastore: %v", err)
	}
	dm := device_manager.NewDeviceManager(ds)
	ab, err := SetupAuthboss(ds)
	if err != nil {
		t.Fatalf("failed to setup authboss: %v", err)
	}
	authService, err := NewAuthService(ab)
	if err != nil {
		t.Fatalf("failed to setup auth service: %v", err)
	}

	return &webServer{
		dataStore:     ds,
		deviceManager: dm,
		auth:          authService,
		captcha:       &TurnstileService{Enabled: false},
	}
}

func TestAPICaptchaConfigHandler(t *testing.T) {
	ws, _, _ := newTestWS(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/captcha/config", nil)
	ws.apiCaptchaConfigHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	env := mustJSONEnvelope(t, rec)
	data := env.Data.(map[string]interface{})
	if data["provider"] != "none" {
		t.Fatalf("unexpected provider: %v", data["provider"])
	}

	ws.captcha = &TurnstileService{Enabled: true, SiteKey: "site_key_x"}
	rec = httptest.NewRecorder()
	ws.apiCaptchaConfigHandler(rec, req)
	env = mustJSONEnvelope(t, rec)
	data = env.Data.(map[string]interface{})
	if data["provider"] != "turnstile" || data["site_key"] != "site_key_x" {
		t.Fatalf("unexpected turnstile config: %+v", data)
	}
}

func TestAPIAuthHandlersValidation(t *testing.T) {
	ws, _, _ := newTestWS(t)

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
		ws.apiRegisterHandler(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("register expected 400, got %d", rec.Code)
		}
		if got := mustJSONEnvelope(t, rec).Code; got != tc.wantCode {
			t.Fatalf("register unexpected code: got %d want %d", got, tc.wantCode)
		}
	}

	ws.captcha = &TurnstileService{Enabled: true}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		bytes.NewBufferString(`{"username":"validuser","password":"Admin123!"}`))
	ws.apiRegisterHandler(rec, req)
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
		ws.apiLoginHandler(rec, req)
		if rec.Code != tc.wantHTTP {
			t.Fatalf("login expected %d, got %d", tc.wantHTTP, rec.Code)
		}
		if got := mustJSONEnvelope(t, rec).Code; got != tc.wantCode {
			t.Fatalf("login unexpected code: got %d want %d", got, tc.wantCode)
		}
	}
}

func TestAPILogoutAndMeAndAuthMiddleware(t *testing.T) {
	ws, _, _ := newTestWS(t)
	user := &datastore.AuthUser{
		Username:   "admin",
		Email:      "admin@test.local",
		Permission: int(inter.PermissionAdmin),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	ws.apiLogoutHandler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("logout expected 401 without session, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req = req.WithContext(context.WithValue(req.Context(), authboss.CTXKeyUser, user))
	req = req.WithContext(context.WithValue(req.Context(), apiCtxUsername, "admin"))
	req = req.WithContext(context.WithValue(req.Context(), apiCtxPerm, inter.PermissionAdmin))
	rec = httptest.NewRecorder()
	ws.apiMeHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("me expected 200, got %d", rec.Code)
	}
	meEnv := mustJSONEnvelope(t, rec)
	meData := meEnv.Data.(map[string]interface{})
	if meData["username"] != "admin" || meData["email"] != "admin@test.local" {
		t.Fatalf("unexpected me data: %+v", meData)
	}

	unauth := ws.apiAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
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
	ws.apiAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next should not run for forbidden request")
	}, inter.PermissionAdmin).ServeHTTP(rec, forbiddenReq)
	if mustJSONEnvelope(t, rec).Code != 40301 {
		t.Fatalf("expected 40301 for forbidden")
	}

	nextCalled := false
	okReq := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	okReq = okReq.WithContext(context.WithValue(okReq.Context(), authboss.CTXKeyUser, user))
	rec = httptest.NewRecorder()
	ws.apiAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		if got := r.Context().Value(apiCtxUsername); got != "admin" {
			t.Fatalf("unexpected context username: %v", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}, inter.PermissionReadOnly).ServeHTTP(rec, okReq)
	if !nextCalled || rec.Code != http.StatusNoContent {
		t.Fatalf("expected middleware to pass request")
	}
}

func TestAPIDeviceAndMetricsAndUsersHandlers(t *testing.T) {
	ws, ds, dm := newTestWS(t)

	uuid := strings.Repeat("a", 64)
	seedDevice(t, ds, uuid, inter.Authenticated)
	if err := ds.AppendMetric(uuid, inter.MetricPoint{
		Timestamp: time.Now().UnixMilli(),
		Value:     26.5,
		Type:      1,
	}); err != nil {
		t.Fatalf("seed metric failed: %v", err)
	}
	seedUser(t, ds, "admin_seed")
	seedUser(t, ds, "tester")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices?status=all&page=1&size=10", nil)
	ws.apiDevicesHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("devices list expected 200, got %d", rec.Code)
	}
	if code := mustJSONEnvelope(t, rec).Code; code != 0 {
		t.Fatalf("devices list expected code 0, got %d", code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/devices/"+uuid, nil)
	rec = httptest.NewRecorder()
	ws.apiDeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("device detail expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/devices/"+uuid+"/approve", nil)
	req = ctxWithPerm(req, inter.PermissionReadWrite)
	rec = httptest.NewRecorder()
	ws.apiDeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/devices/"+uuid+"/token/refresh", nil)
	req = ctxWithPerm(req, inter.PermissionReadWrite)
	rec = httptest.NewRecorder()
	ws.apiDeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh token expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/devices/"+uuid+"/commands",
		bytes.NewBufferString(`{"command":"action_exec","payload":{"op":"reboot"}}`))
	req = ctxWithPerm(req, inter.PermissionReadWrite)
	rec = httptest.NewRecorder()
	ws.apiDeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("enqueue command expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	env := mustJSONEnvelope(t, rec)
	data, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected enqueue response data: %#v", env.Data)
	}
	if data["status"] != string(inter.DeviceCommandStatusQueued) {
		t.Fatalf("unexpected command status: %v", data["status"])
	}

	msg, ok := dm.QueuePop(uuid)
	if !ok {
		t.Fatal("queued command should be available in device queue")
	}
	dmsg, ok := msg.(inter.DownlinkMessage)
	if !ok {
		t.Fatalf("unexpected queued message type: %T", msg)
	}
	if dmsg.CmdID != inter.CmdActionExec || dmsg.CommandID <= 0 {
		t.Fatalf("unexpected queued message: %+v", dmsg)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/metrics/"+uuid+"?range=all", nil)
	rec = httptest.NewRecorder()
	ws.apiMetricsV1Handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	rec = httptest.NewRecorder()
	ws.apiUsersHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("users expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/tester/permission", bytes.NewBufferString(`{"permission":1}`))
	rec = httptest.NewRecorder()
	ws.apiUserPermissionHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update permission expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/devices/"+uuid, nil)
	req = ctxWithPerm(req, inter.PermissionReadWrite)
	rec = httptest.NewRecorder()
	ws.apiDeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete expected 204, got %d", rec.Code)
	}
}

func TestAPIDeviceCommandValidation(t *testing.T) {
	ws, ds, _ := newTestWS(t)
	uuid := strings.Repeat("e", 64)
	seedDevice(t, ds, uuid, inter.Authenticated)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/"+uuid+"/commands",
		bytes.NewBufferString(`{"command":"reboot","payload":{"op":"now"}}`))
	req = ctxWithPerm(req, inter.PermissionReadWrite)
	rec := httptest.NewRecorder()
	ws.apiDeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid command should return 400, got %d", rec.Code)
	}
	if code := mustJSONEnvelope(t, rec).Code; code != 40027 {
		t.Fatalf("invalid command should return code 40027, got %d", code)
	}
}

func TestAPIAuthRegisterLoginLogoutFlow(t *testing.T) {
	ws := newAuthFlowWS(t)

	register := ws.auth.LoadClientStateMiddleware(http.HandlerFunc(ws.apiRegisterHandler))
	login := ws.auth.LoadClientStateMiddleware(http.HandlerFunc(ws.apiLoginHandler))
	logout := ws.auth.LoadClientStateMiddleware(http.HandlerFunc(ws.apiLogoutHandler))

	registerRec := httptest.NewRecorder()
	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		bytes.NewBufferString(`{"username":"admin","password":"Admin123!","email":"admin@test.local"}`))
	register.ServeHTTP(registerRec, registerReq)

	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register expected 201, got %d, body=%s", registerRec.Code, registerRec.Body.String())
	}
	if code := mustJSONEnvelope(t, registerRec).Code; code != 0 {
		t.Fatalf("register business code expected 0, got %d", code)
	}

	conflictRec := httptest.NewRecorder()
	conflictReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		bytes.NewBufferString(`{"username":"admin","password":"Admin123!"}`))
	register.ServeHTTP(conflictRec, conflictReq)
	if conflictRec.Code != http.StatusConflict {
		t.Fatalf("duplicate register expected 409, got %d", conflictRec.Code)
	}
	if code := mustJSONEnvelope(t, conflictRec).Code; code != 40901 {
		t.Fatalf("duplicate register expected code 40901, got %d", code)
	}

	loginRec := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(`{"username":"admin","password":"Admin123!","remember_me":false}`))
	login.ServeHTTP(loginRec, loginReq)
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
	login.ServeHTTP(badLoginRec, badLoginReq)
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
	logout.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout expected 204, got %d", logoutRec.Code)
	}
}

func TestAPIDeviceDeleteNotFound(t *testing.T) {
	ws, _, _ := newTestWS(t)
	uuid := strings.Repeat("b", 64)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/"+uuid, nil)
	req = ctxWithPerm(req, inter.PermissionReadWrite)
	rec := httptest.NewRecorder()

	ws.apiDeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete not-found should return 404, got %d", rec.Code)
	}
}

func TestAPIRefreshTokenNotFound(t *testing.T) {
	ws, _, _ := newTestWS(t)
	uuid := strings.Repeat("c", 64)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/"+uuid+"/token/refresh", nil)
	req = ctxWithPerm(req, inter.PermissionReadWrite)
	rec := httptest.NewRecorder()

	ws.apiDeviceByUUIDHandler(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("refresh token not-found should return 404, got %d", rec.Code)
	}
	if code := mustJSONEnvelope(t, rec).Code; code != 40425 {
		t.Fatalf("unexpected business code: %d", code)
	}
}

func TestAPIUserPermissionValidationAndNotFound(t *testing.T) {
	ws, ds, _ := newTestWS(t)
	seedUser(t, ds, "perm_user")
	beforePerm, err := ds.GetUserPermission("perm_user")
	if err != nil {
		t.Fatalf("failed to get initial permission: %v", err)
	}

	// 非法 permission 必须返回 400
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/perm_user/permission",
		bytes.NewBufferString(`{"permission":"bad"}`))
	rec := httptest.NewRecorder()
	ws.apiUserPermissionHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid permission should return 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if perm, err := ds.GetUserPermission("perm_user"); err != nil || perm != beforePerm {
		t.Fatalf("permission should remain unchanged after invalid request: perm=%d err=%v", perm, err)
	}

	// 用户不存在返回 404
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/not_exist/permission",
		bytes.NewBufferString(`{"permission":2}`))
	rec = httptest.NewRecorder()
	ws.apiUserPermissionHandler(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("user not found should return 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAPIDeviceTokenVisibilityByPermission(t *testing.T) {
	ws, ds, _ := newTestWS(t)
	uuid := strings.Repeat("d", 64)
	seedDevice(t, ds, uuid, inter.Authenticated)

	readonlyReq := httptest.NewRequest(http.MethodGet, "/api/v1/devices?status=all&page=1&size=10", nil)
	readonlyReq = ctxWithPerm(readonlyReq, inter.PermissionReadOnly)
	readonlyRec := httptest.NewRecorder()
	ws.apiDevicesHandler(readonlyRec, readonlyReq)
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
	readwriteReq = ctxWithPerm(readwriteReq, inter.PermissionReadWrite)
	readwriteRec := httptest.NewRecorder()
	ws.apiDeviceByUUIDHandler(readwriteRec, readwriteReq)
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
	ws, ds, _ := newTestWS(t)
	seedUser(t, ds, "admin_only")

	// 保护 1：管理员不能自降权
	selfReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/admin_only/permission",
		bytes.NewBufferString(`{"permission":1}`))
	selfReq = ctxWithUserPerm(selfReq, "admin_only", inter.PermissionAdmin)
	selfRec := httptest.NewRecorder()
	ws.apiUserPermissionHandler(selfRec, selfReq)
	if selfRec.Code != http.StatusBadRequest {
		t.Fatalf("self demotion should return 400, got %d", selfRec.Code)
	}
	if code := mustJSONEnvelope(t, selfRec).Code; code != 40046 {
		t.Fatalf("self demotion expected code 40046, got %d", code)
	}

	// 保护 2：最后一个管理员不能被降权
	lastAdminReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/admin_only/permission",
		bytes.NewBufferString(`{"permission":1}`))
	lastAdminReq = ctxWithUserPerm(lastAdminReq, "other_admin", inter.PermissionAdmin)
	lastAdminRec := httptest.NewRecorder()
	ws.apiUserPermissionHandler(lastAdminRec, lastAdminReq)
	if lastAdminRec.Code != http.StatusBadRequest {
		t.Fatalf("last admin demotion should return 400, got %d", lastAdminRec.Code)
	}
	if code := mustJSONEnvelope(t, lastAdminRec).Code; code != 40047 {
		t.Fatalf("last admin demotion expected code 40047, got %d", code)
	}
}
