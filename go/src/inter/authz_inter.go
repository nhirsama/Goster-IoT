package inter

import (
	"context"
	"errors"
)

type PlatformRole string

const (
	PlatformRoleAdmin    PlatformRole = "platform_admin"
	PlatformRoleOperator PlatformRole = "platform_operator"
	PlatformRoleViewer   PlatformRole = "platform_viewer"
)

type TenantRole string

const (
	TenantRoleAdmin TenantRole = "tenant_admin"
	TenantRoleRW    TenantRole = "tenant_rw"
	TenantRoleRO    TenantRole = "tenant_ro"
)

type GroupRole string

const (
	GroupRoleRW GroupRole = "group_rw"
	GroupRoleRO GroupRole = "group_ro"
)

type APIAction string

type Scope struct {
	TenantID string   `json:"tenant_id"`
	GroupIDs []string `json:"group_ids,omitempty"`
}

type Principal struct {
	Username     string                `json:"username"`
	PlatformRole PlatformRole          `json:"platform_role"`
	TenantRoles  map[string]TenantRole `json:"tenant_roles,omitempty"`
}

// RequestPrincipal 描述一次已认证请求在业务系统内的主体视图。
// 认证层只负责确认“是谁”，租户解析器负责补全当前租户与租户角色。
type RequestPrincipal struct {
	Username    string                `json:"username"`
	Permission  PermissionType        `json:"permission"`
	Scope       Scope                 `json:"scope"`
	TenantRoles map[string]TenantRole `json:"tenant_roles,omitempty"`
}

var (
	ErrTenantRequired   = errors.New("authz: tenant id is required")
	ErrCrossTenantScope = errors.New("authz: cross-tenant access denied")
	ErrGroupScopeDenied = errors.New("authz: group scope access denied")
)

type Authorizer interface {
	Authorize(ctx context.Context, principal Principal, action APIAction, scope Scope, resourceID string) error
}
