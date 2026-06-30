package v1

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// TenantsHandler 管理租户主档。
func (api *API) TenantsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		username, _ := r.Context().Value(ContextUsername).(string)
		tenantRoles, err := api.dataStore.GetUserTenantRoles(username)
		if err != nil {
			api.InternalError(w, r, 50051, err)
			return
		}
		tenants, err := api.dataStore.ListUserTenants(username)
		if err != nil {
			api.InternalError(w, r, 50051, err)
			return
		}
		items := make([]map[string]interface{}, 0, len(tenants))
		for _, tenant := range tenants {
			items = append(items, tenantWithRolePayload(tenant, tenantRoles[tenant.ID]))
		}
		api.OK(w, r, map[string]interface{}{
			"items": items,
			"total": len(tenants),
		})
	case http.MethodPost:
		var payload struct {
			Name   string                 `json:"name"`
			Status string                 `json:"status,omitempty"`
			Meta   map[string]interface{} `json:"meta,omitempty"`
		}
		if err := DecodeBody(r, &payload, api.maxAPIBodyBytes()); err != nil {
			api.Error(w, r, http.StatusBadRequest, 40051, "invalid json body",
				&ErrorDetail{Type: "validation_error"})
			return
		}
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			api.Error(w, r, http.StatusBadRequest, 40052, "tenant name is required",
				&ErrorDetail{Type: "validation_error", Field: "name"})
			return
		}
		if !isValidTenantStatus(payload.Status, true) {
			api.Error(w, r, http.StatusBadRequest, 40058, "invalid tenant status",
				&ErrorDetail{Type: "validation_error", Field: "status"})
			return
		}
		username, _ := r.Context().Value(ContextUsername).(string)
		if strings.TrimSpace(username) == "" {
			api.Error(w, r, http.StatusUnauthorized, 40151, "unauthorized",
				&ErrorDetail{Type: "auth_required"})
			return
		}
		tenant, err := api.dataStore.CreateTenantWithOwner(inter.Tenant{
			Name:   name,
			Status: inter.TenantStatus(payload.Status),
			Meta:   payload.Meta,
		}, username)
		if err != nil {
			api.Error(w, r, http.StatusConflict, 40951, "create tenant failed",
				&ErrorDetail{Type: "conflict", Field: "name", Reason: err.Error()})
			return
		}
		api.write(w, http.StatusCreated, Envelope{
			Code:      0,
			Message:   "ok",
			RequestID: api.requestID(r),
			Data:      tenant,
		})
	default:
		api.MethodNotAllowed(w, r)
	}
}

// TenantByIDHandler 管理租户详情与租户成员子路由。
func (api *API) TenantByIDHandler(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	parts := strings.Split(strings.Trim(suffix, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		api.Error(w, r, http.StatusNotFound, 40451, "tenant not found",
			&ErrorDetail{Type: "not_found", Field: "tenant_id"})
		return
	}
	tenantID, err := url.PathUnescape(parts[0])
	if err != nil || strings.TrimSpace(tenantID) == "" {
		api.Error(w, r, http.StatusBadRequest, 40053, "invalid tenant id",
			&ErrorDetail{Type: "validation_error", Field: "tenant_id"})
		return
	}
	tenantID = strings.TrimSpace(tenantID)

	if len(parts) == 1 {
		api.handleTenantDetail(w, r, tenantID)
		return
	}

	if len(parts) >= 2 && parts[1] == "users" {
		api.handleTenantUsers(w, r, tenantID, parts[2:])
		return
	}

	if len(parts) >= 2 && parts[1] == "invitations" {
		if len(parts) != 2 {
			api.Error(w, r, http.StatusNotFound, 40452, "path not found",
				&ErrorDetail{Type: "not_found"})
			return
		}
		if !api.ensureTenantAccess(w, r, tenantID, inter.PermissionAdmin) {
			return
		}
		api.CreateTenantInvitationHandler(w, r, tenantID)
		return
	}

	api.Error(w, r, http.StatusNotFound, 40452, "path not found",
		&ErrorDetail{Type: "not_found"})
}

func (api *API) handleTenantDetail(w http.ResponseWriter, r *http.Request, tenantID string) {
	switch r.Method {
	case http.MethodGet:
		if !api.ensureTenantAccess(w, r, tenantID, inter.PermissionReadOnly) {
			return
		}
		tenant, err := api.dataStore.GetTenant(tenantID)
		if err != nil {
			api.tenantError(w, r, err, 40453)
			return
		}
		api.OK(w, r, tenant)
	case http.MethodPatch:
		if !api.ensureTenantAccess(w, r, tenantID, inter.PermissionAdmin) {
			return
		}
		var payload struct {
			Name   string                 `json:"name,omitempty"`
			Status string                 `json:"status,omitempty"`
			Meta   map[string]interface{} `json:"meta,omitempty"`
		}
		if err := DecodeBody(r, &payload, api.maxAPIBodyBytes()); err != nil {
			api.Error(w, r, http.StatusBadRequest, 40054, "invalid json body",
				&ErrorDetail{Type: "validation_error"})
			return
		}
		if !isValidTenantStatus(payload.Status, true) {
			api.Error(w, r, http.StatusBadRequest, 40058, "invalid tenant status",
				&ErrorDetail{Type: "validation_error", Field: "status"})
			return
		}
		tenant, err := api.dataStore.UpdateTenant(tenantID, inter.Tenant{
			Name:   payload.Name,
			Status: inter.TenantStatus(payload.Status),
			Meta:   payload.Meta,
		})
		if err != nil {
			api.tenantError(w, r, err, 40453)
			return
		}
		api.OK(w, r, tenant)
	default:
		api.MethodNotAllowed(w, r)
	}
}

func (api *API) handleTenantUsers(w http.ResponseWriter, r *http.Request, tenantID string, rest []string) {
	if !api.ensureTenantAccess(w, r, tenantID, inter.PermissionAdmin) {
		return
	}
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			users, err := api.dataStore.ListTenantUsers(tenantID)
			if err != nil {
				api.tenantError(w, r, err, 40454)
				return
			}
			api.OK(w, r, map[string]interface{}{
				"items": users,
				"total": len(users),
			})
		case http.MethodPost:
			var payload struct {
				Username string `json:"username"`
				Role     string `json:"role"`
			}
			if err := DecodeBody(r, &payload, api.maxAPIBodyBytes()); err != nil {
				api.Error(w, r, http.StatusBadRequest, 40055, "invalid json body",
					&ErrorDetail{Type: "validation_error"})
				return
			}
			username := strings.TrimSpace(payload.Username)
			if username == "" {
				api.Error(w, r, http.StatusBadRequest, 40056, "username is required",
					&ErrorDetail{Type: "validation_error", Field: "username"})
				return
			}
			if !isValidTenantRole(payload.Role) {
				api.Error(w, r, http.StatusBadRequest, 40059, "invalid tenant role",
					&ErrorDetail{Type: "validation_error", Field: "role"})
				return
			}
			if _, err := api.dataStore.GetUserPermission(username); err != nil {
				if errors.Is(err, inter.ErrUserNotFound) {
					api.Error(w, r, http.StatusNotFound, 40457, "user not found",
						&ErrorDetail{Type: "not_found", Field: "username"})
					return
				}
				api.InternalError(w, r, 50053, err)
				return
			}

			// 创建邀请而非直接添加
			invitedBy, _ := r.Context().Value(ContextUsername).(string)
			invitation, err := api.dataStore.CreateTenantInvitation(inter.TenantInvitation{
				TenantID:  tenantID,
				Username:  username,
				Role:      inter.TenantRole(payload.Role),
				InvitedBy: invitedBy,
			})
			if err != nil {
				api.tenantError(w, r, err, 40454)
				return
			}

			api.write(w, http.StatusCreated, Envelope{
				Code:      0,
				Message:   "ok",
				RequestID: api.requestID(r),
				Data: map[string]interface{}{
					"action":     "invite_tenant_user",
					"target":     username,
					"invitation": invitation,
					"success":    true,
				},
			})
		default:
			api.MethodNotAllowed(w, r)
		}
		return
	}

	if len(rest) != 1 || rest[0] == "" {
		api.Error(w, r, http.StatusNotFound, 40455, "path not found",
			&ErrorDetail{Type: "not_found"})
		return
	}
	if r.Method != http.MethodDelete {
		api.MethodNotAllowed(w, r)
		return
	}
	username, err := url.PathUnescape(rest[0])
	if err != nil || strings.TrimSpace(username) == "" {
		api.Error(w, r, http.StatusBadRequest, 40057, "invalid username",
			&ErrorDetail{Type: "validation_error", Field: "username"})
		return
	}
	currentUsername, _ := r.Context().Value(ContextUsername).(string)
	if strings.TrimSpace(currentUsername) == strings.TrimSpace(username) {
		api.Error(w, r, http.StatusBadRequest, 40058, "cannot remove yourself from tenant",
			&ErrorDetail{Type: "validation_error", Field: "username"})
		return
	}
	if err := api.dataStore.RemoveTenantUser(tenantID, username); err != nil {
		api.tenantError(w, r, err, 40456)
		return
	}
	api.NoContent(w, r)
}

func (api *API) tenantError(w http.ResponseWriter, r *http.Request, err error, notFoundCode int) {
	if err == nil {
		return
	}
	if errors.Is(err, inter.ErrTenantNotFound) || errors.Is(err, inter.ErrTenantUserNotFound) {
		api.Error(w, r, http.StatusNotFound, notFoundCode, "tenant not found",
			&ErrorDetail{Type: "not_found", Field: "tenant_id"})
		return
	}
	api.InternalError(w, r, 50052, err)
}

func (api *API) ensureTenantAccess(w http.ResponseWriter, r *http.Request, tenantID string, minPerm inter.PermissionType) bool {
	if api.tenantID(r) != tenantID {
		api.Error(w, r, http.StatusForbidden, 40351, "forbidden",
			&ErrorDetail{Type: "tenant_access_denied", Field: "tenant_id"})
		return false
	}
	perm, _ := r.Context().Value(ContextPerm).(inter.PermissionType)
	if perm < minPerm {
		api.Error(w, r, http.StatusForbidden, 40352, "forbidden",
			&ErrorDetail{Type: "permission_denied"})
		return false
	}
	return true
}

func tenantWithRolePayload(tenant inter.Tenant, role inter.TenantRole) map[string]interface{} {
	return map[string]interface{}{
		"id":         tenant.ID,
		"name":       tenant.Name,
		"status":     tenant.Status,
		"created_at": tenant.CreatedAt,
		"updated_at": tenant.UpdatedAt,
		"meta":       tenant.Meta,
		"role":       role,
	}
}

func isValidTenantStatus(status string, allowEmpty bool) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" {
		return allowEmpty
	}
	switch normalized {
	case string(inter.TenantStatusActive), string(inter.TenantStatusSuspended), string(inter.TenantStatusArchived):
		return true
	default:
		return false
	}
}

func isValidTenantRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case string(inter.TenantRoleAdmin), string(inter.TenantRoleRW), string(inter.TenantRoleRO):
		return true
	default:
		return false
	}
}
