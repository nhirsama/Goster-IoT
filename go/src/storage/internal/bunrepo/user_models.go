package bunrepo

import (
	"database/sql"
	"time"

	"github.com/uptrace/bun"
)

type UserRow struct {
	bun.BaseModel `bun:"table:users"`

	ID         int64          `bun:"id,pk"`
	Username   sql.NullString `bun:"username"`
	Permission int            `bun:"permission"`
	CreatedAt  time.Time      `bun:"created_at"`
}

type TenantRoleRow struct {
	bun.BaseModel `bun:"table:tenant_users"`

	TenantID string `bun:"tenant_id"`
	Role     string `bun:"role"`
}
