package v1

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// InvitationsHandler 处理当前用户的待处理邀请列表
func (api *API) InvitationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.MethodNotAllowed(w, r)
		return
	}

	username, _ := r.Context().Value(ContextUsername).(string)
	if strings.TrimSpace(username) == "" {
		api.Error(w, r, http.StatusUnauthorized, 40151, "unauthorized",
			&ErrorDetail{Type: "auth_required"})
		return
	}

	invitations, err := api.dataStore.ListPendingInvitations(username)
	if err != nil {
		api.InternalError(w, r, 50061, err)
		return
	}

	api.OK(w, r, map[string]interface{}{
		"items": invitations,
		"total": len(invitations),
	})
}

// InvitationByIDHandler 处理单个邀请的接受/拒绝操作
func (api *API) InvitationByIDHandler(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/v1/invitations/")
	parts := strings.Split(strings.Trim(suffix, "/"), "/")
	if len(parts) < 2 || parts[0] == "" {
		api.Error(w, r, http.StatusNotFound, 40461, "invitation not found",
			&ErrorDetail{Type: "not_found", Field: "invitation_id"})
		return
	}

	invitationID, err := url.PathUnescape(parts[0])
	if err != nil || strings.TrimSpace(invitationID) == "" {
		api.Error(w, r, http.StatusBadRequest, 40061, "invalid invitation id",
			&ErrorDetail{Type: "validation_error", Field: "invitation_id"})
		return
	}
	invitationID = strings.TrimSpace(invitationID)

	action := parts[1]
	if r.Method != http.MethodPost {
		api.MethodNotAllowed(w, r)
		return
	}

	username, _ := r.Context().Value(ContextUsername).(string)
	if strings.TrimSpace(username) == "" {
		api.Error(w, r, http.StatusUnauthorized, 40151, "unauthorized",
			&ErrorDetail{Type: "auth_required"})
		return
	}

	// 获取邀请并验证所有权
	invitation, err := api.dataStore.GetInvitation(invitationID)
	if err != nil {
		if errors.Is(err, inter.ErrInvitationNotFound) {
			api.Error(w, r, http.StatusNotFound, 40462, "invitation not found",
				&ErrorDetail{Type: "not_found", Field: "invitation_id"})
			return
		}
		api.InternalError(w, r, 50062, err)
		return
	}

	if strings.TrimSpace(invitation.Username) != strings.TrimSpace(username) {
		api.Error(w, r, http.StatusForbidden, 40361, "forbidden",
			&ErrorDetail{Type: "permission_denied"})
		return
	}

	switch action {
	case "accept":
		if err := api.dataStore.AcceptInvitation(invitationID); err != nil {
			if errors.Is(err, inter.ErrInvitationExpired) {
				api.Error(w, r, http.StatusBadRequest, 40062, "invitation expired",
					&ErrorDetail{Type: "validation_error", Field: "invitation_id"})
				return
			}
			if errors.Is(err, inter.ErrInvitationAccepted) {
				api.Error(w, r, http.StatusBadRequest, 40063, "invitation already processed",
					&ErrorDetail{Type: "validation_error", Field: "invitation_id"})
				return
			}
			api.InternalError(w, r, 50063, err)
			return
		}
		api.OK(w, r, map[string]interface{}{
			"action":  "accept",
			"success": true,
		})

	case "reject":
		if err := api.dataStore.RejectInvitation(invitationID); err != nil {
			if errors.Is(err, inter.ErrInvitationAccepted) {
				api.Error(w, r, http.StatusBadRequest, 40063, "invitation already processed",
					&ErrorDetail{Type: "validation_error", Field: "invitation_id"})
				return
			}
			api.InternalError(w, r, 50064, err)
			return
		}
		api.OK(w, r, map[string]interface{}{
			"action":  "reject",
			"success": true,
		})

	default:
		api.Error(w, r, http.StatusNotFound, 40463, "invalid action",
			&ErrorDetail{Type: "not_found", Field: "action"})
	}
}

// CreateTenantInvitationHandler 创建租户邀请（由租户管理员调用）
func (api *API) CreateTenantInvitationHandler(w http.ResponseWriter, r *http.Request, tenantID string) {
	if r.Method != http.MethodPost {
		api.MethodNotAllowed(w, r)
		return
	}

	var payload struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if err := DecodeBody(r, &payload, api.maxAPIBodyBytes()); err != nil {
		api.Error(w, r, http.StatusBadRequest, 40064, "invalid json body",
			&ErrorDetail{Type: "validation_error"})
		return
	}

	username := strings.TrimSpace(payload.Username)
	if username == "" {
		api.Error(w, r, http.StatusBadRequest, 40065, "username is required",
			&ErrorDetail{Type: "validation_error", Field: "username"})
		return
	}

	if !isValidTenantRole(payload.Role) {
		api.Error(w, r, http.StatusBadRequest, 40066, "invalid tenant role",
			&ErrorDetail{Type: "validation_error", Field: "role"})
		return
	}

	// 验证用户存在
	if _, err := api.dataStore.GetUserPermission(username); err != nil {
		if errors.Is(err, inter.ErrUserNotFound) {
			api.Error(w, r, http.StatusNotFound, 40464, "user not found",
				&ErrorDetail{Type: "not_found", Field: "username"})
			return
		}
		api.InternalError(w, r, 50065, err)
		return
	}

	invitedBy, _ := r.Context().Value(ContextUsername).(string)
	invitation, err := api.dataStore.CreateTenantInvitation(inter.TenantInvitation{
		TenantID:  tenantID,
		Username:  username,
		Role:      inter.TenantRole(payload.Role),
		InvitedBy: invitedBy,
	})
	if err != nil {
		api.tenantError(w, r, err, 40465)
		return
	}

	api.write(w, http.StatusCreated, Envelope{
		Code:      0,
		Message:   "ok",
		RequestID: api.requestID(r),
		Data:      invitation,
	})
}
