package identity

import (
	"context"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

const defaultTenantID = "tenant_legacy"

// PrincipalResolver 负责把“已认证用户”解析成“当前请求主体”。
// Authboss 只负责确认用户身份，租户状态由这里在本地系统内维护。
type PrincipalResolver interface {
	Resolve(ctx context.Context, user inter.SessionUser, requestedTenant string) (inter.RequestPrincipal, error)
}

type tenantRoleStore interface {
	GetUserTenantRoles(username string) (map[string]inter.TenantRole, error)
}

type tenantPrincipalResolver struct {
	dataStore tenantRoleStore
}

func NewTenantPrincipalResolver(dataStore tenantRoleStore) PrincipalResolver {
	return &tenantPrincipalResolver{dataStore: dataStore}
}

func (r *tenantPrincipalResolver) Resolve(
	_ context.Context,
	user inter.SessionUser,
	requestedTenant string,
) (inter.RequestPrincipal, error) {
	if user == nil {
		return inter.RequestPrincipal{}, inter.ErrCrossTenantScope
	}

	requestedTenant = NormalizeTenantID(requestedTenant)
	roles, err := r.loadTenantRoles(user.GetUsername())
	if err != nil {
		return inter.RequestPrincipal{}, err
	}

	tenantID, err := resolveTenantID(user.GetPermission(), requestedTenant, roles)
	if err != nil {
		return inter.RequestPrincipal{}, err
	}

	return inter.RequestPrincipal{
		Username:   user.GetUsername(),
		Permission: user.GetPermission(),
		Scope: inter.Scope{
			TenantID: tenantID,
		},
		TenantRoles: roles,
	}, nil
}

func (r *tenantPrincipalResolver) loadTenantRoles(username string) (map[string]inter.TenantRole, error) {
	username = strings.TrimSpace(username)
	if username == "" || r == nil || r.dataStore == nil {
		return map[string]inter.TenantRole{}, nil
	}

	roles, err := r.dataStore.GetUserTenantRoles(username)
	if err != nil {
		return nil, err
	}
	if roles == nil {
		return map[string]inter.TenantRole{}, nil
	}
	return roles, nil
}

// NormalizeTenantID 统一处理请求中的租户 ID 文本。
func NormalizeTenantID(raw string) string {
	return strings.TrimSpace(raw)
}

func resolveTenantID(perm inter.PermissionType, requestedTenant string, roles map[string]inter.TenantRole) (string, error) {
	if perm >= inter.PermissionAdmin {
		if requestedTenant != "" {
			return requestedTenant, nil
		}
		return chooseDefaultTenant(roles), nil
	}

	if requestedTenant != "" {
		if _, ok := roles[requestedTenant]; ok {
			return requestedTenant, nil
		}
		return "", inter.ErrCrossTenantScope
	}

	tenantID := chooseDefaultTenant(roles)
	if tenantID == "" {
		return "", inter.ErrTenantRequired
	}
	if _, ok := roles[tenantID]; !ok {
		return "", inter.ErrTenantRequired
	}
	return tenantID, nil
}

func chooseDefaultTenant(roles map[string]inter.TenantRole) string {
	if len(roles) == 0 {
		return defaultTenantID
	}
	if _, ok := roles[defaultTenantID]; ok {
		return defaultTenantID
	}

	first := ""
	for tenantID := range roles {
		tenantID = NormalizeTenantID(tenantID)
		if tenantID == "" {
			continue
		}
		if first == "" || tenantID < first {
			first = tenantID
		}
	}
	if first != "" {
		return first
	}
	return defaultTenantID
}
