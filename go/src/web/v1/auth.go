package v1

import (
	"context"
	"net"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/aarondl/authboss/v3"
	"github.com/aarondl/authboss/v3/defaults"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

// CaptchaConfigHandler 返回认证表单所需的公开验证码配置。
func (api *API) CaptchaConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.MethodNotAllowed(w, r)
		return
	}

	provider := "none"
	enabled := false
	siteKey := ""
	if api.captcha != nil && api.captcha.IsEnabled() {
		provider = "turnstile"
		enabled = true
		siteKey = api.captcha.PublicSiteKey()
	}

	api.OK(w, r, map[string]interface{}{
		"provider": provider,
		"enabled":  enabled,
		"site_key": siteKey,
	})
}

// RegisterHandler 创建用户账号，并在成功后建立登录会话。
func (api *API) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.MethodNotAllowed(w, r)
		return
	}

	var payload struct {
		Username     string                 `json:"username"`
		Password     string                 `json:"password"`
		Email        *string                `json:"email,omitempty"`
		CaptchaToken *string                `json:"captcha_token,omitempty"`
		Extensions   map[string]interface{} `json:"extensions,omitempty"`
	}
	if err := DecodeBody(r, &payload, api.maxAPIBodyBytes()); err != nil {
		api.Error(w, r, http.StatusBadRequest, 40001, "invalid json body",
			&ErrorDetail{Type: "validation_error"})
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
		api.Error(w, r, http.StatusBadRequest, 40002, "validation failed",
			&ErrorDetail{Type: "validation_error", Field: "username"})
		return
	}
	if len(username) < 3 || len(username) > 64 {
		api.Error(w, r, http.StatusBadRequest, 40002, "validation failed",
			&ErrorDetail{Type: "validation_error", Field: "username", Reason: "length must be 3..64"})
		return
	}
	if password == "" {
		api.Error(w, r, http.StatusBadRequest, 40003, "validation failed",
			&ErrorDetail{Type: "validation_error", Field: "password"})
		return
	}
	if len(password) < 8 || len(password) > 128 {
		api.Error(w, r, http.StatusBadRequest, 40003, "validation failed",
			&ErrorDetail{Type: "validation_error", Field: "password", Reason: "length must be 8..128"})
		return
	}
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			api.Error(w, r, http.StatusBadRequest, 40002, "validation failed",
				&ErrorDetail{Type: "validation_error", Field: "email", Reason: "invalid email format"})
			return
		}
	}

	if api.captcha != nil && api.captcha.IsEnabled() {
		if captchaToken == "" {
			api.Error(w, r, http.StatusBadRequest, 40005, "captcha token required",
				&ErrorDetail{Type: "validation_error", Field: "captcha_token"})
			return
		}
		if !api.captcha.VerifyToken(captchaToken, clientIPFromRequest(r)) {
			api.Error(w, r, http.StatusBadRequest, 40006, "captcha verification failed",
				&ErrorDetail{Type: "validation_error", Field: "captcha_token"})
			return
		}
	}

	user, err := api.auth.NewAuthableUser(r.Context())
	if err != nil {
		api.InternalError(w, r, 50002, err)
		return
	}
	user.PutPID(username)
	if emailSetter, ok := user.(interface{ PutEmail(string) }); ok {
		emailSetter.PutEmail(email)
	}

	pass, err := api.auth.HashPassword(password)
	if err != nil {
		api.Error(w, r, http.StatusInternalServerError, 50003, "failed to generate password hash",
			&ErrorDetail{Type: "internal_error"})
		return
	}
	user.PutPassword(pass)

	if err := api.auth.CreateUser(r.Context(), user); err != nil {
		if err == authboss.ErrUserFound {
			api.Error(w, r, http.StatusConflict, 40901, "user already exists",
				&ErrorDetail{Type: "conflict", Field: "username"})
			return
		}
		api.InternalError(w, r, 50004, err)
		return
	}

	r = r.WithContext(context.WithValue(r.Context(), authboss.CTXKeyUser, user))
	handled, err := api.auth.FireAfter(authboss.EventRegister, w, r)
	if err != nil {
		api.InternalError(w, r, 50005, err)
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

	api.write(w, http.StatusCreated, Envelope{
		Code:      0,
		Message:   "created",
		RequestID: api.requestID(r),
		Data: map[string]interface{}{
			"username":      username,
			"email":         email,
			"permission":    perm,
			"authenticated": true,
		},
	})
}

// LoginHandler 校验用户名密码，并在成功后写入登录会话。
func (api *API) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.MethodNotAllowed(w, r)
		return
	}

	var payload struct {
		Username     string                 `json:"username"`
		Password     string                 `json:"password"`
		RememberMe   bool                   `json:"remember_me,omitempty"`
		CaptchaToken *string                `json:"captcha_token,omitempty"`
		Extensions   map[string]interface{} `json:"extensions,omitempty"`
	}
	if err := DecodeBody(r, &payload, api.maxAPIBodyBytes()); err != nil {
		api.Error(w, r, http.StatusBadRequest, 40007, "invalid json body",
			&ErrorDetail{Type: "validation_error"})
		return
	}

	username := strings.TrimSpace(payload.Username)
	password := payload.Password
	if username == "" || password == "" {
		field := "username"
		if username != "" {
			field = "password"
		}
		api.Error(w, r, http.StatusBadRequest, 40008, "validation failed",
			&ErrorDetail{Type: "validation_error", Field: field})
		return
	}

	retryAfter, allowed, err := api.loginGuard.Allow(username, r.RemoteAddr)
	if err != nil {
		api.InternalError(w, r, 50014, err)
		return
	}
	if !allowed {
		retryAfterSecs := retryAfterSeconds(retryAfter)
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSecs))
		api.Error(w, r, http.StatusTooManyRequests, 42911, "too many login attempts",
			&ErrorDetail{
				Type:  "too_many_attempts",
				Field: "username",
				Details: map[string]interface{}{
					"retry_after_seconds": retryAfterSecs,
				},
			})
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
		api.Error(w, r, http.StatusBadRequest, 40007, "invalid json body",
			&ErrorDetail{Type: "validation_error"})
		return
	}
	creds := authboss.MustHaveUserValues(validatable)

	pidUser, err := api.auth.LoadUser(r.Context(), creds.GetPID())
	if err == authboss.ErrUserNotFound {
		if err := api.loginGuard.RecordFailure(username, r.RemoteAddr); err != nil {
			api.InternalError(w, r, 50015, err)
			return
		}
		api.Error(w, r, http.StatusUnauthorized, 40111, "invalid credentials",
			&ErrorDetail{Type: "invalid_credentials"})
		return
	}
	if err != nil {
		api.InternalError(w, r, 50006, err)
		return
	}

	authUser, ok := pidUser.(authboss.AuthableUser)
	if !ok {
		api.Error(w, r, http.StatusInternalServerError, 50007, "invalid user type",
			&ErrorDetail{Type: "internal_error"})
		return
	}

	if err := api.auth.VerifyPassword(authUser, creds.GetPassword()); err != nil {
		if storeErr := api.loginGuard.RecordFailure(username, r.RemoteAddr); storeErr != nil {
			api.InternalError(w, r, 50016, storeErr)
			return
		}
		_, _ = api.auth.FireAfter(authboss.EventAuthFail, w, r.WithContext(context.WithValue(r.Context(), authboss.CTXKeyUser, pidUser)))
		api.Error(w, r, http.StatusUnauthorized, 40112, "invalid credentials",
			&ErrorDetail{Type: "invalid_credentials"})
		return
	}
	if err := api.loginGuard.Reset(username, r.RemoteAddr); err != nil {
		api.InternalError(w, r, 50017, err)
		return
	}

	r = r.WithContext(context.WithValue(r.Context(), authboss.CTXKeyUser, pidUser))
	r = r.WithContext(context.WithValue(r.Context(), authboss.CTXKeyValues, validatable))

	handled, err := api.auth.FireBefore(authboss.EventAuth, w, r)
	if err != nil {
		api.InternalError(w, r, 50008, err)
		return
	}
	if handled {
		return
	}

	handled, err = api.auth.FireBefore(authboss.EventAuthHijack, w, r)
	if err != nil {
		api.InternalError(w, r, 50009, err)
		return
	}
	if handled {
		return
	}

	authboss.PutSession(w, authboss.SessionKey, pidUser.GetPID())
	authboss.DelSession(w, authboss.SessionHalfAuthKey)

	handled, err = api.auth.FireAfter(authboss.EventAuth, w, r)
	if err != nil {
		api.InternalError(w, r, 50010, err)
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

	api.OK(w, r, map[string]interface{}{
		"username":      pidUser.GetPID(),
		"email":         email,
		"permission":    perm,
		"authenticated": true,
	})
}

// LogoutHandler 清理当前用户的会话和 remember-me 凭据。
func (api *API) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.MethodNotAllowed(w, r)
		return
	}

	user, err := api.auth.CurrentUser(r)
	if err != nil || user == nil {
		api.Error(w, r, http.StatusUnauthorized, 40113, "unauthorized",
			&ErrorDetail{Type: "auth_required"})
		return
	}

	if err := api.auth.ClearRememberTokens(r.Context(), user.GetPID()); err != nil {
		api.InternalError(w, r, 50012, err)
		return
	}

	authboss.DelKnownSession(w)
	authboss.DelKnownCookie(w)
	api.NoContent(w, r)
}

// MeHandler 返回当前登录用户资料和生效中的租户上下文。
func (api *API) MeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.MethodNotAllowed(w, r)
		return
	}

	username, _ := r.Context().Value(ContextUsername).(string)
	perm, _ := r.Context().Value(ContextPerm).(inter.PermissionType)
	email := ""
	u, _ := api.auth.CurrentUser(r)
	if su, ok := u.(inter.SessionUser); ok {
		email = su.GetEmail()
	}

	api.OK(w, r, map[string]interface{}{
		"username":      username,
		"email":         email,
		"permission":    int(perm),
		"active_tenant": api.tenantID(r),
		"authenticated": true,
	})
}

func clientIPFromRequest(r *http.Request) string {
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	if host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
