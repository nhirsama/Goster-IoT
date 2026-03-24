package datastore

import (
	"context"
	"database/sql"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

const legacyTenantID = "tenant_legacy"

func (s *DataStoreSql) GetUserTenantRoles(username string) (map[string]inter.TenantRole, error) {
	rows, err := s.db.Query(`SELECT tenant_id, role FROM tenant_users WHERE username = ?`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make(map[string]inter.TenantRole)
	for rows.Next() {
		var tenantID string
		var role string
		if err := rows.Scan(&tenantID, &role); err != nil {
			return nil, err
		}
		tenantID = strings.TrimSpace(tenantID)
		if tenantID == "" {
			continue
		}
		roles[tenantID] = normalizeTenantRole(role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roles, nil
}

func permissionToTenantRole(perm inter.PermissionType) inter.TenantRole {
	switch perm {
	case inter.PermissionAdmin:
		return inter.TenantRoleAdmin
	case inter.PermissionReadWrite:
		return inter.TenantRoleRW
	default:
		return inter.TenantRoleRO
	}
}

func normalizeTenantRole(raw string) inter.TenantRole {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(inter.TenantRoleAdmin):
		return inter.TenantRoleAdmin
	case string(inter.TenantRoleRW):
		return inter.TenantRoleRW
	default:
		return inter.TenantRoleRO
	}
}

func (s *DataStoreSql) syncLegacyTenantRoleTx(ctx context.Context, tx *sql.Tx, username string, perm inter.PermissionType) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO tenants (id, name, status) VALUES (?, ?, 'active')`,
		legacyTenantID, "legacy",
	); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO tenant_users (tenant_id, username, role)
		VALUES (?, ?, ?)
		ON CONFLICT(tenant_id, username) DO UPDATE SET role = excluded.role`,
		legacyTenantID, username, string(permissionToTenantRole(perm)),
	)
	return err
}
