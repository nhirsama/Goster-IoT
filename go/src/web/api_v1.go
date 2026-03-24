package web

import (
	"net/http"

	"github.com/nhirsama/Goster-IoT/src/inter"
	apiv1 "github.com/nhirsama/Goster-IoT/src/web/v1"
)

type apiCtxKey = apiv1.ContextKey

const (
	apiCtxRequestID = apiv1.ContextRequestID
	apiCtxUsername  = apiv1.ContextUsername
	apiCtxPerm      = apiv1.ContextPerm
	apiCtxTenantID  = apiv1.ContextTenantID
)

type apiErrorDetail = apiv1.ErrorDetail
type apiEnvelope = apiv1.Envelope

func (ws *webServer) apiV1Handler() *apiv1.API {
	if ws.loginGuard == nil {
		ws.loginGuard = apiv1.NewLoginAttemptGuard(ws.webConfig().LoginProtection)
	}
	deps := apiv1.Deps{
		DataStore:     ws.dataStore,
		DeviceManager: ws.deviceManager,
		API:           ws.api,
		Auth:          ws.auth,
		Captcha:       ws.captcha,
		Logger:        ws.logger,
		Config:        ws.config,
	}
	if ws.apiV1 == nil {
		ws.apiV1 = apiv1.New(deps)
	} else {
		ws.apiV1.SyncDeps(deps)
	}
	ws.apiV1.SetLoginGuardForTest(ws.loginGuard)
	return ws.apiV1
}

func (ws *webServer) registerAPIRoutes(mux *http.ServeMux) {
	ws.apiV1Handler().RegisterRoutes(mux)
}

func (ws *webServer) apiMiddleware(next http.Handler) http.Handler {
	return ws.apiV1Handler().Middleware(next)
}

func (ws *webServer) apiAuthMiddleware(next http.HandlerFunc, minPerm inter.PermissionType) http.Handler {
	return ws.apiV1Handler().AuthMiddleware(next, minPerm)
}

func (ws *webServer) apiCaptchaConfigHandler(w http.ResponseWriter, r *http.Request) {
	ws.apiV1Handler().CaptchaConfigHandler(w, r)
}

func (ws *webServer) apiRegisterHandler(w http.ResponseWriter, r *http.Request) {
	ws.apiV1Handler().RegisterHandler(w, r)
}

func (ws *webServer) apiLoginHandler(w http.ResponseWriter, r *http.Request) {
	ws.apiV1Handler().LoginHandler(w, r)
}

func (ws *webServer) apiLogoutHandler(w http.ResponseWriter, r *http.Request) {
	ws.apiV1Handler().LogoutHandler(w, r)
}

func (ws *webServer) apiMeHandler(w http.ResponseWriter, r *http.Request) {
	ws.apiV1Handler().MeHandler(w, r)
}

func (ws *webServer) apiDevicesHandler(w http.ResponseWriter, r *http.Request) {
	ws.apiV1Handler().DevicesHandler(w, r)
}

func (ws *webServer) apiDeviceByUUIDHandler(w http.ResponseWriter, r *http.Request) {
	ws.apiV1Handler().DeviceByUUIDHandler(w, r)
}

func (ws *webServer) apiMetricsV1Handler(w http.ResponseWriter, r *http.Request) {
	ws.apiV1Handler().MetricsHandler(w, r)
}

func (ws *webServer) apiUsersHandler(w http.ResponseWriter, r *http.Request) {
	ws.apiV1Handler().UsersHandler(w, r)
}

func (ws *webServer) apiUserPermissionHandler(w http.ResponseWriter, r *http.Request) {
	ws.apiV1Handler().UserPermissionHandler(w, r)
}
