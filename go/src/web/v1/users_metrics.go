package v1

import (
	"net/http"
	"strings"
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

// UsersHandler 为当前租户管理员返回账号列表，并附带当前租户角色。
func (api *API) UsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.MethodNotAllowed(w, r)
		return
	}

	users, err := api.dataStore.ListUsersByTenant(api.tenantID(r))
	if err != nil {
		api.InternalError(w, r, 50041, err)
		return
	}

	api.OK(w, r, map[string]interface{}{
		"items": users,
	})
}

// UserPermissionHandler 已废弃。账号权限现在隶属于租户成员关系，请使用租户成员接口更新角色。
func (api *API) UserPermissionHandler(w http.ResponseWriter, r *http.Request) {
	api.Error(w, r, http.StatusGone, 41041, "user permission endpoint is deprecated",
		&ErrorDetail{Type: "deprecated_endpoint", Reason: "use /api/v1/tenants/{tenant_id}/users to manage tenant roles"})
}
