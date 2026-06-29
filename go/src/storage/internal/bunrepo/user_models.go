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

	TenantID  string    `bun:"tenant_id,pk"`
	Username  string    `bun:"username,pk"`
	Role      string    `bun:"role"`
	CreatedAt time.Time `bun:"created_at"`
}

type TenantRow struct {
	bun.BaseModel `bun:"table:tenants"`

	ID        string    `bun:"id,pk"`
	Name      string    `bun:"name"`
	Status    string    `bun:"status"`
	CreatedAt time.Time `bun:"created_at"`
	UpdatedAt time.Time `bun:"updated_at"`
}

type TenantInvitationRow struct {
	bun.BaseModel `bun:"table:tenant_invitations"`

	ID        string    `bun:"id,pk"`
	TenantID  string    `bun:"tenant_id"`
	Username  string    `bun:"username"`
	Role      string    `bun:"role"`
	InvitedBy string    `bun:"invited_by"`
	Status    string    `bun:"status"` // pending, accepted, rejected, expired
	ExpiresAt time.Time `bun:"expires_at"`
	CreatedAt time.Time `bun:"created_at"`
	UpdatedAt time.Time `bun:"updated_at"`
}
