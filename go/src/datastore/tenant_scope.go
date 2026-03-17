package datastore

import "strings"

const defaultTenantID = "tenant_legacy"

func normalizeTenantID(tenantID string) string {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return defaultTenantID
	}
	return tenantID
}
