package v1

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
)

// OK 写入标准的成功响应包体，业务码固定为 `0`。
func (api *API) OK(w http.ResponseWriter, r *http.Request, data interface{}) {
	api.write(w, http.StatusOK, Envelope{
		Code:      0,
		Message:   "ok",
		RequestID: api.requestID(r),
		Data:      data,
	})
}

// Error 使用当前请求 ID 写入结构化错误响应。
func (api *API) Error(w http.ResponseWriter, r *http.Request, httpStatus, code int, message string, detail *ErrorDetail) {
	api.ErrorWithRequestID(w, httpStatus, api.requestID(r), code, message, detail)
}

// ErrorWithRequestID 使用指定请求 ID 写入结构化错误响应。
func (api *API) ErrorWithRequestID(w http.ResponseWriter, httpStatus int, requestID string, code int, message string, detail *ErrorDetail) {
	api.write(w, httpStatus, Envelope{
		Code:      code,
		Message:   message,
		RequestID: requestID,
		Error:     detail,
	})
}

// MethodNotAllowed 返回统一的“方法不允许”响应。
func (api *API) MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	api.Error(w, r, http.StatusMethodNotAllowed, 40501, "method not allowed",
		&ErrorDetail{Type: "method_not_allowed"})
}

// NoContent 返回 `204`，并保留请求 ID 响应头。
func (api *API) NoContent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Request-Id", api.requestID(r))
	w.WriteHeader(http.StatusNoContent)
}

// InternalError 记录服务端错误日志，并返回统一的内部错误响应。
func (api *API) InternalError(w http.ResponseWriter, r *http.Request, code int, err error) {
	requestID := api.requestID(r)
	if err != nil {
		logger.FromContext(r.Context()).Error("Web API 内部错误",
			inter.Int("code", code),
			inter.String("request_id", requestID),
			inter.String("method", r.Method),
			inter.String("path", r.URL.Path),
			inter.Err(err))
	}
	api.ErrorWithRequestID(w, http.StatusInternalServerError, requestID, code, "internal server error",
		&ErrorDetail{Type: "internal_error"})
}

func (api *API) write(w http.ResponseWriter, status int, payload Envelope) {
	if payload.RequestID != "" {
		w.Header().Set("X-Request-Id", payload.RequestID)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (api *API) requestID(r *http.Request) string {
	if rid, ok := r.Context().Value(ContextRequestID).(string); ok && rid != "" {
		return rid
	}
	if rid := r.Header.Get("X-Request-Id"); rid != "" {
		return rid
	}

	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err == nil {
		return "req_" + time.Now().UTC().Format("20060102150405") + "_" + hex.EncodeToString(buf)
	}
	return "req_" + time.Now().UTC().Format("20060102150405.000000000")
}

func countAdminUsers(users []inter.User) int {
	count := 0
	for _, u := range users {
		if u.Permission == inter.PermissionAdmin {
			count++
		}
	}
	return count
}

func canViewDeviceToken(r *http.Request) bool {
	perm, _ := r.Context().Value(ContextPerm).(inter.PermissionType)
	return perm >= inter.PermissionReadWrite
}

func deviceMetadataPayload(meta inter.DeviceMetadata, includeToken bool) map[string]interface{} {
	token := interface{}(nil)
	if includeToken {
		token = meta.Token
	}
	return map[string]interface{}{
		"name":                meta.Name,
		"hw_version":          meta.HWVersion,
		"sw_version":          meta.SWVersion,
		"config_version":      meta.ConfigVersion,
		"sn":                  meta.SerialNumber,
		"mac":                 meta.MACAddress,
		"created_at":          meta.CreatedAt,
		"token":               token,
		"authenticate_status": int(meta.AuthenticateStatus),
	}
}
