package tenant

import (
	"context"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/bunrepo"
	"github.com/uptrace/bun"
)

type Repository struct {
	db *bun.DB
}

func NewRepository(db *bun.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserTenantRoles(username string) (map[string]inter.TenantRole, error) {
	var rows []bunrepo.TenantRoleRow
	if err := r.db.NewSelect().
		Model(&rows).
		Column("tenant_id", "role").
		Where("username = ?", username).
		Scan(context.Background()); err != nil {
		return nil, err
	}

	out := make(map[string]inter.TenantRole, len(rows))
	for _, row := range rows {
		tenantID := strings.TrimSpace(row.TenantID)
		if tenantID == "" {
			continue
		}
		out[tenantID] = bunrepo.NormalizeTenantRole(row.Role)
	}
	return out, nil
}
