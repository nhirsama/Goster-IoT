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

var (
	ErrTenantRequired   = errors.New("authz: tenant id is required")
	ErrCrossTenantScope = errors.New("authz: cross-tenant access denied")
	ErrGroupScopeDenied = errors.New("authz: group scope access denied")
)

type Authorizer interface {
	Authorize(ctx context.Context, principal Principal, action APIAction, scope Scope, resourceID string) error
}
