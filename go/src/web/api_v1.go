package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/aarondl/authboss/v3/defaults"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

type apiCtxKey string

const (
	apiCtxRequestID apiCtxKey = "request_id"
	apiCtxUsername  apiCtxKey = "username"
	apiCtxPerm      apiCtxKey = "permission"
)

const (
	minValidMetricsTimestampMs int64 = 1672531200000
	maxAPIBodyBytes            int64 = 1 << 20
	defaultDevicePageSize            = 100
	maxDevicePageSize                = 1000

	defaultAPICORSAllowedOrigins = "http://localhost:5173,http://127.0.0.1:5173"
)

type apiErrorDetail struct {
	Type    string                 `json:"type"`
	Field   string                 `json:"field,omitempty"`
	Reason  string                 `json:"reason,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type apiEnvelope struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	RequestID string          `json:"request_id"`
	Data      interface{}     `json:"data,omitempty"`
	Error     *apiErrorDetail `json:"error,omitempty"`
	Meta      interface{}     `json:"meta,omitempty"`
}

func (ws *webServer) registerAPIRoutes(mux *http.ServeMux) {
	public := func(h http.HandlerFunc) http.Handler {
		return ws.apiMiddleware(ws.auth.LoadClientStateMiddleware(h))
	}
	protected := func(h http.HandlerFunc, minPerm inter.PermissionType) http.Handler {
		return ws.apiMiddleware(ws.auth.LoadClientStateMiddleware(ws.apiAuthMiddleware(h, minPerm)))
	}

	mux.Handle("/api/v1/auth/captcha/config", public(ws.apiCaptchaConfigHandler))
	mux.Handle("/api/v1/auth/register", public(ws.apiRegisterHandler))
	mux.Handle("/api/v1/auth/login", public(ws.apiLoginHandler))
	mux.Handle("/api/v1/auth/logout", public(ws.apiLogoutHandler))
	mux.Handle("/api/v1/auth/me", protected(ws.apiMeHandler, inter.PermissionNone))

	mux.Handle("/api/v1/devices", protected(ws.apiDevicesHandler, inter.PermissionReadOnly))
	mux.Handle("/api/v1/devices/", protected(ws.apiDeviceByUUIDHandler, inter.PermissionReadOnly))

	mux.Handle("/api/v1/metrics/", protected(ws.apiMetricsV1Handler, inter.PermissionReadOnly))

	mux.Handle("/api/v1/users", protected(ws.apiUsersHandler, inter.PermissionAdmin))
	mux.Handle("/api/v1/users/", protected(ws.apiUserPermissionHandler, inter.PermissionAdmin))
}

func (ws *webServer) apiMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := ws.getRequestID(r)
		r = r.WithContext(context.WithValue(r.Context(), apiCtxRequestID, rid))

		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			allowOrigin, ok := ws.resolveAllowedAPIOrigin(r, origin)
			if !ok {
				ws.apiError(w, r, http.StatusForbidden, 40302, "origin not allowed",
					&apiErrorDetail{Type: "forbidden_origin", Field: "origin"})
				return
			}
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", allowOrigin)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Expose-Headers", "X-Request-Id")
		}

		if r.Method == http.MethodOptions {
			ws.apiNoContent(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (ws *webServer) resolveAllowedAPIOrigin(r *http.Request, origin string) (string, bool) {
	if isSameOriginRequest(r, origin) {
		return origin, true
	}

	raw := strings.TrimSpace(os.Getenv("API_CORS_ALLOW_ORIGINS"))
	if raw == "" {
		raw = defaultAPICORSAllowedOrigins
	}

	for _, candidate := range strings.Split(raw, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if candidate == "*" || candidate == origin {
			return origin, true
		}
	}
	return "", false
}

func isSameOriginRequest(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	host := r.Host
	if forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		host = strings.TrimSpace(strings.Split(forwardedHost, ",")[0])
	}
	if !strings.EqualFold(u.Host, host) {
		return false
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = strings.TrimSpace(strings.Split(forwardedProto, ",")[0])
	}
	return strings.EqualFold(u.Scheme, scheme)
}

func (ws *webServer) apiAuthMiddleware(next http.HandlerFunc, minPerm inter.PermissionType) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := ws.getRequestID(r)

		u, err := ws.auth.CurrentUser(r)
		if err != nil || u == nil {
			ws.apiErrorWithRequestID(
				w, http.StatusUnauthorized, rid, 40101, "unauthorized",
				&apiErrorDetail{Type: "auth_required"},
			)
			return
		}

		user, ok := u.(inter.SessionUser)
		if !ok {
			ws.apiErrorWithRequestID(
				w, http.StatusInternalServerError, rid, 50001, "invalid user type",
				&apiErrorDetail{Type: "internal_error"},
			)
			return
		}

		if user.GetPermission() < minPerm {
			ws.apiErrorWithRequestID(
				w, http.StatusForbidden, rid, 40301, "forbidden",
				&apiErrorDetail{Type: "permission_denied"},
			)
			return
		}

		ctx := context.WithValue(r.Context(), apiCtxRequestID, rid)
		ctx = context.WithValue(ctx, apiCtxUsername, user.GetUsername())
		ctx = context.WithValue(ctx, apiCtxPerm, user.GetPermission())

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (ws *webServer) apiCaptchaConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ws.apiMethodNotAllowed(w, r)
		return
	}

	provider := "none"
	enabled := false
	siteKey := ""
	if ws.captcha != nil && ws.captcha.IsEnabled() {
		provider = "turnstile"
		enabled = true
		siteKey = ws.captcha.PublicSiteKey()
	}

	ws.apiOK(w, r, map[string]interface{}{
		"provider": provider,
		"enabled":  enabled,
		"site_key": siteKey,
	})
}

func (ws *webServer) apiRegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ws.apiMethodNotAllowed(w, r)
		return
	}

	var payload struct {
		Username     string                 `json:"username"`
		Password     string                 `json:"password"`
		Email        *string                `json:"email,omitempty"`
		CaptchaToken *string                `json:"captcha_token,omitempty"`
		Extensions   map[string]interface{} `json:"extensions,omitempty"`
	}
	if err := decodeAPIBody(r, &payload); err != nil {
		ws.apiError(w, r, http.StatusBadRequest, 40001, "invalid json body",
			&apiErrorDetail{Type: "validation_error"})
		return
	}
	username := strings.TrimSpace(payload.Username)
	password := payload.Password
	email := ""
	captchaToken := ""
	if payload.Email != nil {
		email = strings.TrimSpace(*payload.Email)
	}
	if payload.CaptchaToken != nil {
		captchaToken = strings.TrimSpace(*payload.CaptchaToken)
	}

	if username == "" {
		ws.apiError(w, r, http.StatusBadRequest, 40002, "validation failed",
			&apiErrorDetail{Type: "validation_error", Field: "username"})
		return
	}
	if len(username) < 3 || len(username) > 64 {
		ws.apiError(w, r, http.StatusBadRequest, 40002, "validation failed",
			&apiErrorDetail{Type: "validation_error", Field: "username", Reason: "length must be 3..64"})
		return
	}
	if password == "" {
		ws.apiError(w, r, http.StatusBadRequest, 40003, "validation failed",
			&apiErrorDetail{Type: "validation_error", Field: "password"})
		return
	}
	if len(password) < 8 || len(password) > 128 {
		ws.apiError(w, r, http.StatusBadRequest, 40003, "validation failed",
			&apiErrorDetail{Type: "validation_error", Field: "password", Reason: "length must be 8..128"})
		return
	}
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			ws.apiError(w, r, http.StatusBadRequest, 40002, "validation failed",
				&apiErrorDetail{Type: "validation_error", Field: "email", Reason: "invalid email format"})
			return
		}
	}

	if ws.captcha != nil && ws.captcha.IsEnabled() {
		if captchaToken == "" {
			ws.apiError(w, r, http.StatusBadRequest, 40005, "captcha token required",
				&apiErrorDetail{Type: "validation_error", Field: "captcha_token"})
			return
		}
		if !ws.captcha.VerifyToken(captchaToken, clientIPFromRequest(r)) {
			ws.apiError(w, r, http.StatusBadRequest, 40006, "captcha verification failed",
				&apiErrorDetail{Type: "validation_error", Field: "captcha_token"})
			return
		}
	}

	user, err := ws.auth.NewAuthableUser(r.Context())
	if err != nil {
		ws.apiInternalError(w, r, 50002, err)
		return
	}
	user.PutPID(username)

	if emailSetter, ok := user.(interface{ PutEmail(string) }); ok {
		emailSetter.PutEmail(email)
	}

	pass, err := ws.auth.HashPassword(password)
	if err != nil {
		ws.apiError(w, r, http.StatusInternalServerError, 50003, "failed to generate password hash",
			&apiErrorDetail{Type: "internal_error"})
		return
	}
	user.PutPassword(pass)

	if err := ws.auth.CreateUser(r.Context(), user); err != nil {
		if err == authboss.ErrUserFound {
			ws.apiError(w, r, http.StatusConflict, 40901, "user already exists",
				&apiErrorDetail{Type: "conflict", Field: "username"})
			return
		}
		ws.apiInternalError(w, r, 50004, err)
		return
	}

	r = r.WithContext(context.WithValue(r.Context(), authboss.CTXKeyUser, user))
	handled, err := ws.auth.FireAfter(authboss.EventRegister, w, r)
	if err != nil {
		ws.apiInternalError(w, r, 50005, err)
		return
	}
	if !handled {
		authboss.PutSession(w, authboss.SessionKey, user.GetPID())
		authboss.DelSession(w, authboss.SessionHalfAuthKey)
	}

	perm := int(inter.PermissionNone)
	if sessionUser, ok := user.(inter.SessionUser); ok {
		perm = int(sessionUser.GetPermission())
	}

	ws.apiWrite(w, http.StatusCreated, apiEnvelope{
		Code:      0,
		Message:   "created",
		RequestID: ws.getRequestID(r),
		Data: map[string]interface{}{
			"username":      username,
			"email":         email,
			"permission":    perm,
			"authenticated": true,
		},
	})
}

func (ws *webServer) apiLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ws.apiMethodNotAllowed(w, r)
		return
	}

	var payload struct {
		Username     string                 `json:"username"`
		Password     string                 `json:"password"`
		RememberMe   bool                   `json:"remember_me,omitempty"`
		CaptchaToken *string                `json:"captcha_token,omitempty"`
		Extensions   map[string]interface{} `json:"extensions,omitempty"`
	}
	if err := decodeAPIBody(r, &payload); err != nil {
		ws.apiError(w, r, http.StatusBadRequest, 40007, "invalid json body",
			&apiErrorDetail{Type: "validation_error"})
		return
	}
	username := strings.TrimSpace(payload.Username)
	password := payload.Password
	if username == "" || password == "" {
		field := "username"
		if username != "" {
			field = "password"
		}
		ws.apiError(w, r, http.StatusBadRequest, 40008, "validation failed",
			&apiErrorDetail{Type: "validation_error", Field: field})
		return
	}

	values := map[string]string{
		"username": username,
		"password": password,
	}
	if payload.RememberMe {
		values[authboss.CookieRemember] = "true"
	}

	reader := defaults.NewHTTPBodyReader(true, true)
	validatable, err := reader.Read("login", requestWithStringMapJSON(r, values))
	if err != nil {
		ws.apiError(w, r, http.StatusBadRequest, 40007, "invalid json body",
			&apiErrorDetail{Type: "validation_error"})
		return
	}
	creds := authboss.MustHaveUserValues(validatable)

	pidUser, err := ws.auth.LoadUser(r.Context(), creds.GetPID())
	if err == authboss.ErrUserNotFound {
		ws.apiError(w, r, http.StatusUnauthorized, 40111, "invalid credentials",
			&apiErrorDetail{Type: "invalid_credentials"})
		return
	}
	if err != nil {
		ws.apiInternalError(w, r, 50006, err)
		return
	}

	authUser, ok := pidUser.(authboss.AuthableUser)
	if !ok {
		ws.apiError(w, r, http.StatusInternalServerError, 50007, "invalid user type",
			&apiErrorDetail{Type: "internal_error"})
		return
	}

	if err := ws.auth.VerifyPassword(authUser, creds.GetPassword()); err != nil {
		_, _ = ws.auth.FireAfter(authboss.EventAuthFail, w, r.WithContext(context.WithValue(r.Context(), authboss.CTXKeyUser, pidUser)))
		ws.apiError(w, r, http.StatusUnauthorized, 40112, "invalid credentials",
			&apiErrorDetail{Type: "invalid_credentials"})
		return
	}

	r = r.WithContext(context.WithValue(r.Context(), authboss.CTXKeyUser, pidUser))
	r = r.WithContext(context.WithValue(r.Context(), authboss.CTXKeyValues, validatable))

	handled, err := ws.auth.FireBefore(authboss.EventAuth, w, r)
	if err != nil {
		ws.apiInternalError(w, r, 50008, err)
		return
	}
	if handled {
		return
	}

	handled, err = ws.auth.FireBefore(authboss.EventAuthHijack, w, r)
	if err != nil {
		ws.apiInternalError(w, r, 50009, err)
		return
	}
	if handled {
		return
	}

	authboss.PutSession(w, authboss.SessionKey, pidUser.GetPID())
	authboss.DelSession(w, authboss.SessionHalfAuthKey)

	handled, err = ws.auth.FireAfter(authboss.EventAuth, w, r)
	if err != nil {
		ws.apiInternalError(w, r, 50010, err)
		return
	}
	if handled {
		return
	}

	email := ""
	perm := int(inter.PermissionNone)
	if sessionUser, ok := pidUser.(inter.SessionUser); ok {
		email = sessionUser.GetEmail()
		perm = int(sessionUser.GetPermission())
	}

	ws.apiOK(w, r, map[string]interface{}{
		"username":      pidUser.GetPID(),
		"email":         email,
		"permission":    perm,
		"authenticated": true,
	})
}

func (ws *webServer) apiLogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ws.apiMethodNotAllowed(w, r)
		return
	}

	user, err := ws.auth.CurrentUser(r)
	if err != nil || user == nil {
		ws.apiError(w, r, http.StatusUnauthorized, 40113, "unauthorized",
			&apiErrorDetail{Type: "auth_required"})
		return
	}

	if err := ws.auth.ClearRememberTokens(r.Context(), user.GetPID()); err != nil {
		ws.apiInternalError(w, r, 50012, err)
		return
	}

	authboss.DelKnownSession(w)
	authboss.DelKnownCookie(w)
	ws.apiNoContent(w, r)
}

func (ws *webServer) apiMeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ws.apiMethodNotAllowed(w, r)
		return
	}

	username, _ := r.Context().Value(apiCtxUsername).(string)
	perm, _ := r.Context().Value(apiCtxPerm).(inter.PermissionType)

	email := ""
	u, _ := ws.auth.CurrentUser(r)
	if su, ok := u.(inter.SessionUser); ok {
		email = su.GetEmail()
	}

	ws.apiOK(w, r, map[string]interface{}{
		"username":      username,
		"email":         email,
		"permission":    int(perm),
		"authenticated": true,
	})
}

func (ws *webServer) apiDevicesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ws.apiMethodNotAllowed(w, r)
		return
	}

	status, statusPtr, statusErr := parseDeviceStatusFilter(r.URL.Query().Get("status"))
	if statusErr != nil {
		ws.apiError(w, r, http.StatusBadRequest, 40011, "invalid status filter",
			&apiErrorDetail{Type: "validation_error", Field: "status"})
		return
	}

	page, err := parsePositiveIntQuery(r.URL.Query().Get("page"), 1, 0)
	if err != nil {
		ws.apiError(w, r, http.StatusBadRequest, 40012, "invalid page",
			&apiErrorDetail{Type: "validation_error", Field: "page", Reason: err.Error()})
		return
	}
	size, err := parsePositiveIntQuery(r.URL.Query().Get("size"), defaultDevicePageSize, maxDevicePageSize)
	if err != nil {
		ws.apiError(w, r, http.StatusBadRequest, 40013, "invalid size",
			&apiErrorDetail{Type: "validation_error", Field: "size", Reason: err.Error()})
		return
	}

	devices, err := ws.deviceManager.ListDevices(statusPtr, page, size)
	if err != nil {
		ws.apiInternalError(w, r, 50011, err)
		return
	}

	includeToken := ws.apiCanViewDeviceToken(r)
	items := make([]map[string]interface{}, 0, len(devices))
	for _, d := range devices {
		runtimeStatus, _ := ws.deviceManager.QueryDeviceStatus(d.UUID)
		statusText := "offline"
		switch runtimeStatus {
		case inter.StatusOnline:
			statusText = "online"
		case inter.StatusDelayed:
			statusText = "delayed"
		}

		items = append(items, map[string]interface{}{
			"uuid": d.UUID,
			"meta": apiDeviceMetaData(d.Meta, includeToken),
			"runtime": map[string]interface{}{
				"status":      int(runtimeStatus),
				"status_text": statusText,
			},
		})
	}

	ws.apiOK(w, r, map[string]interface{}{
		"items": items,
		"page": map[string]interface{}{
			"page":     page,
			"size":     size,
			"returned": len(items),
		},
		"status_filter": status,
	})
}

func (ws *webServer) apiDeviceByUUIDHandler(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/v1/devices/")
	parts := strings.Split(strings.Trim(suffix, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		ws.apiError(w, r, http.StatusNotFound, 40411, "device not found",
			&apiErrorDetail{Type: "not_found", Field: "uuid"})
		return
	}
	uuid, err := url.PathUnescape(parts[0])
	if err != nil || uuid == "" {
		ws.apiError(w, r, http.StatusBadRequest, 40021, "invalid uuid",
			&apiErrorDetail{Type: "validation_error", Field: "uuid"})
		return
	}

	// /api/v1/devices/{uuid}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			ws.apiGetDevice(w, r, uuid)
		case http.MethodDelete:
			if !ws.ensureAPIPerm(w, r, inter.PermissionReadWrite) {
				return
			}
			if err := ws.deviceManager.DeleteDevice(uuid); err != nil {
				if isNotFoundError(err) {
					ws.apiError(w, r, http.StatusNotFound, 40424, "device not found",
						&apiErrorDetail{Type: "not_found", Field: "uuid"})
					return
				}
				ws.apiInternalError(w, r, 50025, err)
				return
			}
			ws.apiNoContent(w, r)
		default:
			ws.apiMethodNotAllowed(w, r)
		}
		return
	}

	// /api/v1/devices/{uuid}/{action}
	if len(parts) == 2 {
		if r.Method != http.MethodPost {
			ws.apiMethodNotAllowed(w, r)
			return
		}
		if !ws.ensureAPIPerm(w, r, inter.PermissionReadWrite) {
			return
		}
		action := parts[1]
		var err error
		switch action {
		case "approve":
			err = ws.deviceManager.ApproveDevice(uuid)
		case "revoke":
			err = ws.deviceManager.RejectDevice(uuid)
		case "unblock":
			err = ws.deviceManager.UnblockDevice(uuid)
		default:
			ws.apiError(w, r, http.StatusNotFound, 40412, "action not found",
				&apiErrorDetail{Type: "not_found", Field: "action"})
			return
		}
		if err != nil {
			if isNotFoundError(err) {
				ws.apiError(w, r, http.StatusNotFound, 40423, "device not found",
					&apiErrorDetail{Type: "not_found", Field: "uuid"})
				return
			}
			ws.apiInternalError(w, r, 50022, err)
			return
		}

		ws.apiOK(w, r, map[string]interface{}{
			"action":  action,
			"target":  uuid,
			"success": true,
		})
		return
	}

	// /api/v1/devices/{uuid}/token/refresh
	if len(parts) == 3 && parts[1] == "token" && parts[2] == "refresh" {
		if r.Method != http.MethodPost {
			ws.apiMethodNotAllowed(w, r)
			return
		}
		if !ws.ensureAPIPerm(w, r, inter.PermissionReadWrite) {
			return
		}
		token, err := ws.deviceManager.RefreshToken(uuid)
		if err != nil {
			if isNotFoundError(err) {
				ws.apiError(w, r, http.StatusNotFound, 40425, "device not found",
					&apiErrorDetail{Type: "not_found", Field: "uuid"})
				return
			}
			ws.apiInternalError(w, r, 50023, err)
			return
		}

		ws.apiOK(w, r, map[string]interface{}{
			"uuid":       uuid,
			"token":      token,
			"rotated_at": time.Now().UTC(),
		})
		return
	}

	ws.apiError(w, r, http.StatusNotFound, 40413, "path not found",
		&apiErrorDetail{Type: "not_found"})
}

func (ws *webServer) apiGetDevice(w http.ResponseWriter, r *http.Request, uuid string) {
	meta, err := ws.deviceManager.GetDeviceMetadata(uuid)
	if err != nil {
		ws.apiError(w, r, http.StatusNotFound, 40421, "device not found",
			&apiErrorDetail{Type: "not_found", Field: "uuid"})
		return
	}

	runtimeStatus, _ := ws.deviceManager.QueryDeviceStatus(uuid)
	statusText := "offline"
	switch runtimeStatus {
	case inter.StatusOnline:
		statusText = "online"
	case inter.StatusDelayed:
		statusText = "delayed"
	}

	includeToken := ws.apiCanViewDeviceToken(r)
	ws.apiOK(w, r, map[string]interface{}{
		"uuid": uuid,
		"meta": apiDeviceMetaData(meta, includeToken),
		"runtime": map[string]interface{}{
			"status":      int(runtimeStatus),
			"status_text": statusText,
		},
	})
}

func (ws *webServer) apiMetricsV1Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ws.apiMethodNotAllowed(w, r)
		return
	}

	uuid := strings.TrimPrefix(r.URL.Path, "/api/v1/metrics/")
	uuid = strings.Trim(uuid, "/")
	if uuid == "" {
		ws.apiError(w, r, http.StatusNotFound, 40431, "device not found",
			&apiErrorDetail{Type: "not_found", Field: "uuid"})
		return
	}
	if !ws.apiEnsureDeviceExists(w, r, uuid) {
		return
	}

	start, end, rangeLabel, err := resolveMetricsRange(r)
	if err != nil {
		ws.apiError(w, r, http.StatusBadRequest, 40031, err.Error(),
			&apiErrorDetail{Type: "validation_error"})
		return
	}

	points, err := ws.dataStore.QueryMetrics(uuid, start, end)
	if err != nil {
		ws.apiInternalError(w, r, 50031, err)
		return
	}

	ws.apiOK(w, r, map[string]interface{}{
		"uuid":     uuid,
		"range":    rangeLabel,
		"start_ms": start,
		"end_ms":   end,
		"points":   points,
	})
}

func (ws *webServer) apiUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ws.apiMethodNotAllowed(w, r)
		return
	}

	users, err := ws.dataStore.ListUsers()
	if err != nil {
		ws.apiInternalError(w, r, 50041, err)
		return
	}

	ws.apiOK(w, r, map[string]interface{}{
		"items": users,
	})
}

func (ws *webServer) apiUserPermissionHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "permission" {
		ws.apiError(w, r, http.StatusNotFound, 40441, "path not found",
			&apiErrorDetail{Type: "not_found"})
		return
	}
	if r.Method != http.MethodPost {
		ws.apiMethodNotAllowed(w, r)
		return
	}

	username, err := url.PathUnescape(parts[0])
	if err != nil || strings.TrimSpace(username) == "" {
		ws.apiError(w, r, http.StatusBadRequest, 40043, "invalid username",
			&apiErrorDetail{Type: "validation_error", Field: "username"})
		return
	}

	var payload struct {
		Permission int                    `json:"permission"`
		Extensions map[string]interface{} `json:"extensions,omitempty"`
	}
	if err := decodeAPIBody(r, &payload); err != nil {
		ws.apiError(w, r, http.StatusBadRequest, 40044, "invalid json body",
			&apiErrorDetail{Type: "validation_error", Field: "permission"})
		return
	}
	if payload.Permission < int(inter.PermissionNone) || payload.Permission > int(inter.PermissionAdmin) {
		ws.apiError(w, r, http.StatusBadRequest, 40045, "invalid permission",
			&apiErrorDetail{Type: "validation_error", Field: "permission"})
		return
	}

	currentUsername, _ := r.Context().Value(apiCtxUsername).(string)
	if currentUsername != "" && strings.EqualFold(currentUsername, username) && payload.Permission < int(inter.PermissionAdmin) {
		ws.apiError(w, r, http.StatusBadRequest, 40046, "cannot change your own admin permission",
			&apiErrorDetail{Type: "validation_error", Field: "permission"})
		return
	}

	currentPerm, err := ws.dataStore.GetUserPermission(username)
	if err != nil {
		if isNotFoundError(err) {
			ws.apiError(w, r, http.StatusNotFound, 40442, "user not found",
				&apiErrorDetail{Type: "not_found", Field: "username"})
			return
		}
		ws.apiInternalError(w, r, 50043, err)
		return
	}

	if currentPerm == inter.PermissionAdmin && payload.Permission < int(inter.PermissionAdmin) {
		users, err := ws.dataStore.ListUsers()
		if err != nil {
			ws.apiInternalError(w, r, 50044, err)
			return
		}
		if countAdminUsers(users) <= 1 {
			ws.apiError(w, r, http.StatusBadRequest, 40047, "cannot demote the last admin",
				&apiErrorDetail{Type: "validation_error", Field: "permission"})
			return
		}
	}

	if err := ws.dataStore.UpdateUserPermission(username, inter.PermissionType(payload.Permission)); err != nil {
		if isNotFoundError(err) {
			ws.apiError(w, r, http.StatusNotFound, 40442, "user not found",
				&apiErrorDetail{Type: "not_found", Field: "username"})
			return
		}
		ws.apiInternalError(w, r, 50042, err)
		return
	}

	ws.apiOK(w, r, map[string]interface{}{
		"action":  "update_permission",
		"target":  username,
		"success": true,
	})
}

func (ws *webServer) ensureAPIPerm(w http.ResponseWriter, r *http.Request, minPerm inter.PermissionType) bool {
	perm, _ := r.Context().Value(apiCtxPerm).(inter.PermissionType)
	if perm < minPerm {
		ws.apiError(w, r, http.StatusForbidden, 40311, "forbidden",
			&apiErrorDetail{Type: "permission_denied"})
		return false
	}
	return true
}

func (ws *webServer) apiEnsureDeviceExists(w http.ResponseWriter, r *http.Request, uuid string) bool {
	if _, err := ws.deviceManager.GetDeviceMetadata(uuid); err != nil {
		if isNotFoundError(err) {
			ws.apiError(w, r, http.StatusNotFound, 40421, "device not found",
				&apiErrorDetail{Type: "not_found", Field: "uuid"})
			return false
		}
		ws.apiInternalError(w, r, 50024, err)
		return false
	}
	return true
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}

func decodeAPIBody(r *http.Request, out interface{}) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxAPIBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}

	var tail struct{}
	if err := dec.Decode(&tail); err != io.EOF {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func requestWithStringMapJSON(r *http.Request, values map[string]string) *http.Request {
	payload, _ := json.Marshal(values)
	rr := r.Clone(r.Context())
	rr.Body = io.NopCloser(bytes.NewReader(payload))
	rr.ContentLength = int64(len(payload))
	return rr
}

func parseDeviceStatusFilter(raw string) (string, *inter.AuthenticateStatusType, error) {
	status := strings.TrimSpace(strings.ToLower(raw))
	if status == "" {
		status = "authenticated"
	}

	switch status {
	case "all":
		return status, nil, nil
	case "authenticated":
		v := inter.Authenticated
		return status, &v, nil
	case "pending":
		v := inter.AuthenticatePending
		return status, &v, nil
	case "refused":
		v := inter.AuthenticateRefuse
		return status, &v, nil
	case "revoked":
		v := inter.AuthenticateRevoked
		return status, &v, nil
	default:
		return "", nil, strconv.ErrSyntax
	}
}

func parsePositiveIntQuery(raw string, fallback int, max int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New("must be an integer")
	}
	if v <= 0 {
		return 0, errors.New("must be greater than 0")
	}
	if max > 0 && v > max {
		return 0, fmt.Errorf("must be less than or equal to %d", max)
	}
	return v, nil
}

func resolveMetricsRange(r *http.Request) (start int64, end int64, rangeLabel string, err error) {
	end = time.Now().UnixMilli()
	rangeLabel = r.URL.Query().Get("range")
	if rangeLabel == "" {
		rangeLabel = "1h"
	}
	if !isValidMetricsRange(rangeLabel) {
		return 0, 0, "", errors.New("invalid range")
	}

	startRaw := r.URL.Query().Get("start_ms")
	endRaw := r.URL.Query().Get("end_ms")

	// start_ms/end_ms 必须成对提供
	if startRaw != "" || endRaw != "" {
		if startRaw == "" || endRaw == "" {
			return 0, 0, "", errors.New("start_ms and end_ms must be provided together")
		}
		parsedStart, startErr := strconv.ParseInt(startRaw, 10, 64)
		parsedEnd, endErr := strconv.ParseInt(endRaw, 10, 64)
		if startErr != nil || endErr != nil {
			return 0, 0, "", errors.New("start_ms and end_ms must be integers")
		}
		if parsedStart > parsedEnd {
			return 0, 0, "", errors.New("start_ms must be less than or equal to end_ms")
		}
		start = parsedStart
		end = parsedEnd
		if start < minValidMetricsTimestampMs {
			start = minValidMetricsTimestampMs
		}
		return start, end, rangeLabel, nil
	}

	switch rangeLabel {
	case "all":
		start = minValidMetricsTimestampMs
	case "1h":
		start = time.Now().Add(-time.Hour).UnixMilli()
	case "6h":
		start = time.Now().Add(-6 * time.Hour).UnixMilli()
	case "24h":
		start = time.Now().Add(-24 * time.Hour).UnixMilli()
	case "7d":
		start = time.Now().Add(-7 * 24 * time.Hour).UnixMilli()
	}

	if start < minValidMetricsTimestampMs {
		start = minValidMetricsTimestampMs
	}
	return start, end, rangeLabel, nil
}

func isValidMetricsRange(raw string) bool {
	switch raw {
	case "1h", "6h", "24h", "7d", "all":
		return true
	default:
		return false
	}
}

func (ws *webServer) apiOK(w http.ResponseWriter, r *http.Request, data interface{}) {
	ws.apiWrite(w, http.StatusOK, apiEnvelope{
		Code:      0,
		Message:   "ok",
		RequestID: ws.getRequestID(r),
		Data:      data,
	})
}

func (ws *webServer) apiError(w http.ResponseWriter, r *http.Request, httpStatus, code int, message string, detail *apiErrorDetail) {
	ws.apiErrorWithRequestID(w, httpStatus, ws.getRequestID(r), code, message, detail)
}

func (ws *webServer) apiErrorWithRequestID(w http.ResponseWriter, httpStatus int, requestID string, code int, message string, detail *apiErrorDetail) {
	ws.apiWrite(w, httpStatus, apiEnvelope{
		Code:      code,
		Message:   message,
		RequestID: requestID,
		Error:     detail,
	})
}

func (ws *webServer) apiMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	ws.apiError(w, r, http.StatusMethodNotAllowed, 40501, "method not allowed",
		&apiErrorDetail{Type: "method_not_allowed"})
}

func (ws *webServer) apiNoContent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Request-Id", ws.getRequestID(r))
	w.WriteHeader(http.StatusNoContent)
}

func (ws *webServer) apiWrite(w http.ResponseWriter, status int, payload apiEnvelope) {
	if payload.RequestID != "" {
		w.Header().Set("X-Request-Id", payload.RequestID)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (ws *webServer) getRequestID(r *http.Request) string {
	if rid, ok := r.Context().Value(apiCtxRequestID).(string); ok && rid != "" {
		return rid
	}
	if rid := r.Header.Get("X-Request-Id"); rid != "" {
		return rid
	}

	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err == nil {
		return "req_" + strconv.FormatInt(time.Now().Unix(), 10) + "_" + hex.EncodeToString(buf)
	}
	return "req_" + strconv.FormatInt(time.Now().UnixNano(), 10)
}
