package v1

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// DevicesHandler 在当前租户范围内分页列出设备，并支持状态过滤。
func (api *API) DevicesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.MethodNotAllowed(w, r)
		return
	}

	status, statusPtr, statusErr := ParseDeviceStatusFilter(r.URL.Query().Get("status"))
	if statusErr != nil {
		api.Error(w, r, http.StatusBadRequest, 40011, "invalid status filter",
			&ErrorDetail{Type: "validation_error", Field: "status"})
		return
	}

	page, err := ParsePositiveIntQuery(r.URL.Query().Get("page"), 1, 0)
	if err != nil {
		api.Error(w, r, http.StatusBadRequest, 40012, "invalid page",
			&ErrorDetail{Type: "validation_error", Field: "page", Reason: err.Error()})
		return
	}
	size, err := ParsePositiveIntQuery(r.URL.Query().Get("size"), api.deviceListDefaultPageSize(), api.deviceListMaxPageSize())
	if err != nil {
		api.Error(w, r, http.StatusBadRequest, 40013, "invalid size",
			&ErrorDetail{Type: "validation_error", Field: "size", Reason: err.Error()})
		return
	}

	devices, err := api.registry.ListDevicesByScope(api.scopeFromRequest(r), statusPtr, page, size)
	if err != nil {
		api.InternalError(w, r, 50011, err)
		return
	}

	includeToken := canViewDeviceToken(r)
	items := make([]map[string]interface{}, 0, len(devices))
	for _, d := range devices {
		runtimeStatus, _ := api.presence.QueryDeviceStatus(d.UUID)
		statusText := "offline"
		switch runtimeStatus {
		case inter.StatusOnline:
			statusText = "online"
		case inter.StatusDelayed:
			statusText = "delayed"
		}

		items = append(items, map[string]interface{}{
			"uuid": d.UUID,
			"meta": deviceMetadataPayload(d.Meta, includeToken),
			"runtime": map[string]interface{}{
				"status":      int(runtimeStatus),
				"status_text": statusText,
			},
		})
	}

	api.OK(w, r, map[string]interface{}{
		"items": items,
		"page": map[string]interface{}{
			"page":     page,
			"size":     size,
			"returned": len(items),
		},
		"status_filter": status,
	})
}

// DeviceByUUIDHandler 负责分发设备详情、控制动作和命令子路由。
func (api *API) DeviceByUUIDHandler(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/v1/devices/")
	parts := strings.Split(strings.Trim(suffix, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		api.Error(w, r, http.StatusNotFound, 40411, "device not found",
			&ErrorDetail{Type: "not_found", Field: "uuid"})
		return
	}
	uuid, err := url.PathUnescape(parts[0])
	if err != nil || uuid == "" {
		api.Error(w, r, http.StatusBadRequest, 40021, "invalid uuid",
			&ErrorDetail{Type: "validation_error", Field: "uuid"})
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			api.getDevice(w, r, uuid)
		case http.MethodDelete:
			if !api.ensurePerm(w, r, inter.PermissionReadWrite) || !api.ensureDeviceInScope(w, r, uuid, 40424) {
				return
			}
			if err := api.registry.DeleteDevice(uuid); err != nil {
				if isNotFoundError(err) {
					api.Error(w, r, http.StatusNotFound, 40424, "device not found",
						&ErrorDetail{Type: "not_found", Field: "uuid"})
					return
				}
				api.InternalError(w, r, 50025, err)
				return
			}
			api.NoContent(w, r)
		default:
			api.MethodNotAllowed(w, r)
		}
		return
	}

	if len(parts) == 2 {
		if r.Method != http.MethodPost {
			api.MethodNotAllowed(w, r)
			return
		}
		if !api.ensurePerm(w, r, inter.PermissionReadWrite) || !api.ensureDeviceInScope(w, r, uuid, 40423) {
			return
		}
		action := parts[1]
		var err error
		switch action {
		case "approve":
			err = api.registry.ApproveDevice(uuid)
		case "revoke":
			err = api.registry.RejectDevice(uuid)
		case "unblock":
			err = api.registry.UnblockDevice(uuid)
		case "commands":
			api.enqueueDeviceCommand(w, r, uuid)
			return
		default:
			api.Error(w, r, http.StatusNotFound, 40412, "action not found",
				&ErrorDetail{Type: "not_found", Field: "action"})
			return
		}
		if err != nil {
			if isNotFoundError(err) {
				api.Error(w, r, http.StatusNotFound, 40423, "device not found",
					&ErrorDetail{Type: "not_found", Field: "uuid"})
				return
			}
			api.InternalError(w, r, 50022, err)
			return
		}
		api.OK(w, r, map[string]interface{}{
			"action":  action,
			"target":  uuid,
			"success": true,
		})
		return
	}

	if len(parts) == 3 && parts[1] == "token" && parts[2] == "refresh" {
		if r.Method != http.MethodPost {
			api.MethodNotAllowed(w, r)
			return
		}
		if !api.ensurePerm(w, r, inter.PermissionReadWrite) || !api.ensureDeviceInScope(w, r, uuid, 40425) {
			return
		}
		token, err := api.registry.RefreshToken(uuid)
		if err != nil {
			if isNotFoundError(err) {
				api.Error(w, r, http.StatusNotFound, 40425, "device not found",
					&ErrorDetail{Type: "not_found", Field: "uuid"})
				return
			}
			api.InternalError(w, r, 50023, err)
			return
		}
		api.OK(w, r, map[string]interface{}{
			"uuid":       uuid,
			"token":      token,
			"rotated_at": time.Now().UTC(),
		})
		return
	}

	api.Error(w, r, http.StatusNotFound, 40413, "path not found",
		&ErrorDetail{Type: "not_found"})
}

func (api *API) getDevice(w http.ResponseWriter, r *http.Request, uuid string) {
	meta, err := api.registry.GetDeviceMetadataByScope(api.scopeFromRequest(r), uuid)
	if err != nil {
		api.deviceScopeError(w, r, err, 40421)
		return
	}

	runtimeStatus, _ := api.presence.QueryDeviceStatus(uuid)
	statusText := "offline"
	switch runtimeStatus {
	case inter.StatusOnline:
		statusText = "online"
	case inter.StatusDelayed:
		statusText = "delayed"
	}

	api.OK(w, r, map[string]interface{}{
		"uuid": uuid,
		"meta": deviceMetadataPayload(meta, canViewDeviceToken(r)),
		"runtime": map[string]interface{}{
			"status":      int(runtimeStatus),
			"status_text": statusText,
		},
	})
}

func (api *API) enqueueDeviceCommand(w http.ResponseWriter, r *http.Request, uuid string) {
	if !api.ensureDeviceExists(w, r, uuid) {
		return
	}

	var payload struct {
		Command string          `json:"command"`
		Payload json.RawMessage `json:"payload,omitempty"`
	}
	if err := DecodeBody(r, &payload, api.maxAPIBodyBytes()); err != nil {
		api.Error(w, r, http.StatusBadRequest, 40026, "invalid json body",
			&ErrorDetail{Type: "validation_error"})
		return
	}

	cmdID, command, err := ParseDownlinkCommand(payload.Command)
	if err != nil {
		api.Error(w, r, http.StatusBadRequest, 40027, "invalid command",
			&ErrorDetail{Type: "validation_error", Field: "command"})
		return
	}

	rawPayload := []byte(strings.TrimSpace(string(payload.Payload)))
	scope := api.scopeFromRequest(r)
	msg, err := api.downlinkCommands.Enqueue(scope, uuid, cmdID, command, rawPayload)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "tenant mismatch") {
			api.Error(w, r, http.StatusForbidden, 40321, "forbidden",
				&ErrorDetail{Type: "cross_tenant_denied"})
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "队列") || strings.Contains(strings.ToLower(err.Error()), "queue") {
			api.Error(w, r, http.StatusConflict, 40921, "queue command failed",
				&ErrorDetail{Type: "conflict", Field: "command"})
			return
		}
		api.Error(w, r, http.StatusConflict, 40921, "queue command failed",
			&ErrorDetail{Type: "conflict", Field: "command"})
		return
	}

	api.OK(w, r, map[string]interface{}{
		"command_id":  msg.CommandID,
		"uuid":        uuid,
		"command":     command,
		"cmd_id":      int(cmdID),
		"status":      inter.DeviceCommandStatusQueued,
		"enqueued_at": time.Now().UTC(),
	})
}

func (api *API) ensurePerm(w http.ResponseWriter, r *http.Request, minPerm inter.PermissionType) bool {
	perm, _ := r.Context().Value(ContextPerm).(inter.PermissionType)
	if perm < minPerm {
		api.Error(w, r, http.StatusForbidden, 40311, "forbidden",
			&ErrorDetail{Type: "permission_denied"})
		return false
	}
	return true
}

func (api *API) ensureDeviceExists(w http.ResponseWriter, r *http.Request, uuid string) bool {
	return api.ensureDeviceInScope(w, r, uuid, 40421)
}

func (api *API) ensureDeviceInScope(w http.ResponseWriter, r *http.Request, uuid string, notFoundCode int) bool {
	if _, err := api.registry.GetDeviceMetadataByScope(api.scopeFromRequest(r), uuid); err != nil {
		api.deviceScopeError(w, r, err, notFoundCode)
		return false
	}
	return true
}

func (api *API) deviceScopeError(w http.ResponseWriter, r *http.Request, err error, notFoundCode int) {
	if err == nil {
		return
	}
	lowerErr := strings.ToLower(err.Error())
	if strings.Contains(lowerErr, "tenant mismatch") {
		api.Error(w, r, http.StatusForbidden, 40321, "forbidden",
			&ErrorDetail{Type: "cross_tenant_denied", Field: "uuid"})
		return
	}
	if strings.Contains(lowerErr, "not found") {
		api.Error(w, r, http.StatusNotFound, notFoundCode, "device not found",
			&ErrorDetail{Type: "not_found", Field: "uuid"})
		return
	}
	api.InternalError(w, r, 50024, err)
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}
