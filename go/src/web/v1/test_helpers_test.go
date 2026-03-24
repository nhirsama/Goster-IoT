package v1_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/aarondl/authboss/v3"
	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/datastore"
	"github.com/nhirsama/Goster-IoT/src/device_manager"
	"github.com/nhirsama/Goster-IoT/src/inter"
	webpkg "github.com/nhirsama/Goster-IoT/src/web"
	apiv1 "github.com/nhirsama/Goster-IoT/src/web/v1"
)

type apiTestOptions struct {
	config     appcfg.WebConfig
	captcha    *webpkg.TurnstileService
	loginGuard *apiv1.LoginAttemptGuard
}

type apiTestEnv struct {
	api           *apiv1.API
	auth          webpkg.AuthService
	dataStore     inter.DataStore
	deviceManager inter.DeviceManager
}

func newTestAPI(t *testing.T, opts ...apiTestOptions) *apiTestEnv {
	t.Helper()

	option := apiTestOptions{
		config:  appcfg.DefaultWebConfig(),
		captcha: &webpkg.TurnstileService{},
	}
	if len(opts) > 0 {
		option = opts[0]
		if option.captcha == nil {
			option.captcha = &webpkg.TurnstileService{}
		}
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")
	ds, err := datastore.NewDataStoreSql(dbPath)
	if err != nil {
		t.Fatalf("failed to init datastore: %v", err)
	}
	dm := device_manager.NewDeviceManager(ds)
	ab, err := webpkg.SetupAuthboss(ds)
	if err != nil {
		t.Fatalf("failed to setup authboss: %v", err)
	}
	authService, err := webpkg.NewAuthService(ab)
	if err != nil {
		t.Fatalf("failed to setup auth service: %v", err)
	}

	api := apiv1.New(apiv1.Deps{
		DataStore:     ds,
		DeviceManager: dm,
		Auth:          authService,
		Captcha:       option.captcha,
		Config:        option.config,
		LoginGuard:    option.loginGuard,
	})

	return &apiTestEnv{
		api:           api,
		auth:          authService,
		dataStore:     ds,
		deviceManager: dm,
	}
}

func newTestMux(api *apiv1.API) *http.ServeMux {
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	return mux
}

func mustJSONEnvelope(t *testing.T, rec *httptest.ResponseRecorder) apiv1.Envelope {
	t.Helper()
	var env apiv1.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode envelope: %v, body=%s", err, rec.Body.String())
	}
	return env
}

func withPerm(req *http.Request, perm inter.PermissionType) *http.Request {
	ctx := context.WithValue(req.Context(), apiv1.ContextPerm, perm)
	ctx = context.WithValue(ctx, apiv1.ContextRequestID, "req_test")
	return req.WithContext(ctx)
}

func withUserPerm(req *http.Request, username string, perm inter.PermissionType) *http.Request {
	ctx := context.WithValue(req.Context(), apiv1.ContextUsername, username)
	ctx = context.WithValue(ctx, apiv1.ContextPerm, perm)
	ctx = context.WithValue(ctx, apiv1.ContextRequestID, "req_test")
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

func registerAPITestUser(t *testing.T, mux *http.ServeMux, username, password string) {
	t.Helper()
	body := `{"username":"` + username + `","password":"` + password + `","email":"` + username + `@test.local"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register %s failed: status=%d body=%s", username, rec.Code, rec.Body.String())
	}
}

func loginAPITestUser(t *testing.T, mux *http.ServeMux, username, password, remoteAddr string) []*http.Cookie {
	t.Helper()
	body := `{"username":"` + username + `","password":"` + password + `","remember_me":false}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	req.RemoteAddr = remoteAddr
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s failed: status=%d body=%s", username, rec.Code, rec.Body.String())
	}
	return rec.Result().Cookies()
}
