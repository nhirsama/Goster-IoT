package v1

import (
	"errors"
	"net/http"

	"github.com/nhirsama/Goster-IoT/src/identity"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

func (api *API) requestedTenantID(r *http.Request) string {
	return identity.NormalizeTenantID(r.Header.Get("X-Tenant-Id"))
}

func (api *API) tenantID(r *http.Request) string {
	if ctxTenant, ok := r.Context().Value(ContextTenantID).(string); ok {
		if tenantID := identity.NormalizeTenantID(ctxTenant); tenantID != "" {
			return tenantID
		}
	}
	if requested := api.requestedTenantID(r); requested != "" {
		return requested
	}
	return inter.DefaultTenantID
}

func (api *API) scopeFromRequest(r *http.Request) inter.Scope {
	return inter.Scope{TenantID: api.tenantID(r)}
}

func isTenantAccessError(err error) bool {
	return errors.Is(err, inter.ErrTenantRequired) || errors.Is(err, inter.ErrCrossTenantScope)
}
