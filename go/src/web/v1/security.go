package v1

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

const defaultTenantID = "tenant_legacy"

type tenantAccessResolver struct {
	dataStore inter.DataStore
}

func newTenantAccessResolver(dataStore inter.DataStore) *tenantAccessResolver {
	return &tenantAccessResolver{dataStore: dataStore}
}

func (r *tenantAccessResolver) Resolve(
	ctx context.Context,
	username string,
	perm inter.PermissionType,
	requestedTenant string,
) (inter.Scope, map[string]inter.TenantRole, error) {
	requestedTenant = normalizeTenantID(requestedTenant)
	roles, err := r.loadTenantRoles(ctx, username)
	if err != nil {
		return inter.Scope{}, nil, err
	}

	if perm >= inter.PermissionAdmin {
		return inter.Scope{TenantID: resolveAdminTenant(requestedTenant, roles)}, roles, nil
	}

	activeTenant, ok := resolveMemberTenant(requestedTenant, roles)
	if !ok {
		if requestedTenant == "" {
			return inter.Scope{}, roles, inter.ErrTenantRequired
		}
		return inter.Scope{}, roles, inter.ErrCrossTenantScope
	}
	return inter.Scope{TenantID: activeTenant}, roles, nil
}

func (r *tenantAccessResolver) loadTenantRoles(_ context.Context, username string) (map[string]inter.TenantRole, error) {
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

func (api *API) requestedTenantID(r *http.Request) string {
	return normalizeTenantID(r.Header.Get("X-Tenant-Id"))
}

func (api *API) tenantID(r *http.Request) string {
	if ctxTenant, ok := r.Context().Value(ContextTenantID).(string); ok {
		if tenantID := normalizeTenantID(ctxTenant); tenantID != "" {
			return tenantID
		}
	}
	if requested := api.requestedTenantID(r); requested != "" {
		return requested
	}
	return defaultTenantID
}

func (api *API) scopeFromRequest(r *http.Request) inter.Scope {
	return inter.Scope{TenantID: api.tenantID(r)}
}

func normalizeTenantID(raw string) string {
	return strings.TrimSpace(raw)
}

func resolveAdminTenant(requestedTenant string, roles map[string]inter.TenantRole) string {
	if requestedTenant != "" {
		return requestedTenant
	}
	return chooseDefaultTenant(roles)
}

func resolveMemberTenant(requestedTenant string, roles map[string]inter.TenantRole) (string, bool) {
	if requestedTenant != "" {
		_, ok := roles[requestedTenant]
		return requestedTenant, ok
	}
	activeTenant := chooseDefaultTenant(roles)
	if activeTenant == "" {
		return "", false
	}
	_, ok := roles[activeTenant]
	return activeTenant, ok
}

func chooseDefaultTenant(roles map[string]inter.TenantRole) string {
	if len(roles) == 0 {
		return defaultTenantID
	}
	if _, ok := roles[defaultTenantID]; ok {
		return defaultTenantID
	}
	tenantIDs := make([]string, 0, len(roles))
	for tenantID := range roles {
		tenantID = normalizeTenantID(tenantID)
		if tenantID != "" {
			tenantIDs = append(tenantIDs, tenantID)
		}
	}
	if len(tenantIDs) == 0 {
		return defaultTenantID
	}
	sort.Strings(tenantIDs)
	return tenantIDs[0]
}

func isTenantAccessError(err error) bool {
	return errors.Is(err, inter.ErrTenantRequired) || errors.Is(err, inter.ErrCrossTenantScope)
}
