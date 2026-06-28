package v1

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// MetricsHandler 返回当前租户范围内设备的时序指标数据。
func (api *API) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.MethodNotAllowed(w, r)
		return
	}

	uuid := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/metrics/"), "/")
	if uuid == "" {
		api.Error(w, r, http.StatusNotFound, 40431, "device not found",
			&ErrorDetail{Type: "not_found", Field: "uuid"})
		return
	}
	if !api.ensureDeviceExists(w, r, uuid) {
		return
	}

	start, end, rangeLabel, err := ResolveMetricsRange(r, api.metricsMinValidTimestampMs(), api.metricsDefaultRangeLabel())
	if err != nil {
		api.Error(w, r, http.StatusBadRequest, 40031, err.Error(),
			&ErrorDetail{Type: "validation_error"})
		return
	}

	points, err := api.dataStore.QueryMetricsByTenant(api.tenantID(r), uuid, start, end)
	if err != nil {
		api.InternalError(w, r, 50031, err)
		return
	}

	api.OK(w, r, map[string]interface{}{
		"uuid":     uuid,
		"range":    rangeLabel,
		"start_ms": start,
		"end_ms":   end,
		"points":   points,
	})
}

// UsersHandler 为管理员返回平台用户列表。
func (api *API) UsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.MethodNotAllowed(w, r)
		return
	}

	users, err := api.dataStore.ListUsers()
	if err != nil {
		api.InternalError(w, r, 50041, err)
		return
	}

	api.OK(w, r, map[string]interface{}{
		"items": users,
	})
}

// UserPermissionHandler 更新指定用户的平台权限等级。
func (api *API) UserPermissionHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "permission" {
		api.Error(w, r, http.StatusNotFound, 40441, "path not found",
			&ErrorDetail{Type: "not_found"})
		return
	}
	if r.Method != http.MethodPost {
		api.MethodNotAllowed(w, r)
		return
	}

	username, err := url.PathUnescape(parts[0])
	if err != nil || strings.TrimSpace(username) == "" {
		api.Error(w, r, http.StatusBadRequest, 40043, "invalid username",
			&ErrorDetail{Type: "validation_error", Field: "username"})
		return
	}

	var payload struct {
		Permission int                    `json:"permission"`
		Extensions map[string]interface{} `json:"extensions,omitempty"`
	}
	if err := DecodeBody(r, &payload, api.maxAPIBodyBytes()); err != nil {
		api.Error(w, r, http.StatusBadRequest, 40044, "invalid json body",
			&ErrorDetail{Type: "validation_error", Field: "permission"})
		return
	}
	if payload.Permission < int(inter.PermissionNone) || payload.Permission > int(inter.PermissionAdmin) {
		api.Error(w, r, http.StatusBadRequest, 40045, "invalid permission",
			&ErrorDetail{Type: "validation_error", Field: "permission"})
		return
	}

	currentUsername, _ := r.Context().Value(ContextUsername).(string)
	if currentUsername != "" && strings.EqualFold(currentUsername, username) && payload.Permission < int(inter.PermissionAdmin) {
		api.Error(w, r, http.StatusBadRequest, 40046, "cannot change your own admin permission",
			&ErrorDetail{Type: "validation_error", Field: "permission"})
		return
	}

	currentPerm, err := api.dataStore.GetUserPermission(username)
	if err != nil {
		if isNotFoundError(err) {
			api.Error(w, r, http.StatusNotFound, 40442, "user not found",
				&ErrorDetail{Type: "not_found", Field: "username"})
			return
		}
		api.InternalError(w, r, 50043, err)
		return
	}

	if currentPerm == inter.PermissionAdmin && payload.Permission < int(inter.PermissionAdmin) {
		users, err := api.dataStore.ListUsers()
		if err != nil {
			api.InternalError(w, r, 50044, err)
			return
		}
		if countAdminUsers(users) <= 1 {
			api.Error(w, r, http.StatusBadRequest, 40047, "cannot demote the last admin",
				&ErrorDetail{Type: "validation_error", Field: "permission"})
			return
		}
	}

	if err := api.dataStore.UpdateUserPermission(username, inter.PermissionType(payload.Permission)); err != nil {
		if isNotFoundError(err) {
			api.Error(w, r, http.StatusNotFound, 40442, "user not found",
				&ErrorDetail{Type: "not_found", Field: "username"})
			return
		}
		api.InternalError(w, r, 50042, err)
		return
	}

	api.OK(w, r, map[string]interface{}{
		"action":  "update_permission",
		"target":  username,
		"success": true,
	})
}
