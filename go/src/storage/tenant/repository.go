package tenant

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

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

func (r *Repository) ListTenants() ([]inter.Tenant, error) {
	var rows []bunrepo.TenantRow
	if err := r.db.NewSelect().
		Model(&rows).
		OrderExpr("created_at ASC, id ASC").
		Scan(context.Background()); err != nil {
		return nil, err
	}

	out := make([]inter.Tenant, 0, len(rows))
	for _, row := range rows {
		out = append(out, tenantFromRow(row))
	}
	return out, nil
}

func (r *Repository) ListUserTenants(username string) ([]inter.Tenant, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return []inter.Tenant{}, nil
	}
	var rows []bunrepo.TenantRow
	if err := r.db.NewSelect().
		Model(&rows).
		Join("JOIN tenant_users AS tu ON tu.tenant_id = tenant_row.id").
		Where("tu.username = ?", username).
		OrderExpr("tenant_row.created_at ASC, tenant_row.id ASC").
		Scan(context.Background()); err != nil {
		return nil, err
	}

	out := make([]inter.Tenant, 0, len(rows))
	for _, row := range rows {
		out = append(out, tenantFromRow(row))
	}
	return out, nil
}

func (r *Repository) GetTenant(tenantID string) (inter.Tenant, error) {
	var row bunrepo.TenantRow
	err := r.db.NewSelect().
		Model(&row).
		Where("id = ?", bunrepo.NormalizeTenantID(tenantID)).
		Limit(1).
		Scan(context.Background())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return inter.Tenant{}, inter.ErrTenantNotFound
		}
		return inter.Tenant{}, err
	}
	return tenantFromRow(row), nil
}

func (r *Repository) CreateTenant(tenant inter.Tenant) (inter.Tenant, error) {
	row, err := newTenantRow(tenant)
	if err != nil {
		return inter.Tenant{}, err
	}
	if _, err := r.db.NewInsert().
		Model(row).
		Returning("NULL").
		Exec(context.Background()); err != nil {
		return inter.Tenant{}, err
	}
	return tenantFromRow(*row), nil
}

func (r *Repository) CreateTenantWithOwner(tenant inter.Tenant, ownerUsername string) (inter.Tenant, error) {
	ownerUsername = strings.TrimSpace(ownerUsername)
	if ownerUsername == "" {
		return inter.Tenant{}, errors.New("owner username is required")
	}
	row, err := newTenantRow(tenant)
	if err != nil {
		return inter.Tenant{}, err
	}
	err = r.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().
			Model(row).
			Returning("NULL").
			Exec(ctx); err != nil {
			return err
		}
		return upsertTenantUser(ctx, tx, row.ID, ownerUsername, inter.TenantRoleAdmin, row.CreatedAt)
	})
	if err != nil {
		return inter.Tenant{}, err
	}
	return tenantFromRow(*row), nil
}

func newTenantRow(tenant inter.Tenant) (*bunrepo.TenantRow, error) {
	name := strings.TrimSpace(tenant.Name)
	if name == "" {
		return nil, errors.New("tenant name is required")
	}
	status := normalizeTenantStatus(tenant.Status)
	id := strings.TrimSpace(tenant.ID)
	if id == "" {
		id = slugTenantID(name)
	}
	now := time.Now().UTC()
	row := &bunrepo.TenantRow{
		ID:        id,
		Name:      name,
		Status:    string(status),
		CreatedAt: now,
		UpdatedAt: now,
	}
	return row, nil
}

func (r *Repository) UpdateTenant(tenantID string, updates inter.Tenant) (inter.Tenant, error) {
	tenantID = bunrepo.NormalizeTenantID(tenantID)
	name := strings.TrimSpace(updates.Name)
	status := normalizeTenantStatus(updates.Status)
	query := r.db.NewUpdate().
		Model((*bunrepo.TenantRow)(nil)).
		Set("updated_at = ?", time.Now().UTC()).
		Where("id = ?", tenantID)
	if name != "" {
		query = query.Set("name = ?", name)
	}
	if updates.Status != "" {
		query = query.Set("status = ?", string(status))
	}
	res, err := query.Returning("NULL").Exec(context.Background())
	if err != nil {
		return inter.Tenant{}, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return inter.Tenant{}, err
	}
	if rows == 0 {
		return inter.Tenant{}, inter.ErrTenantNotFound
	}
	return r.GetTenant(tenantID)
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

func (r *Repository) ListTenantUsers(tenantID string) ([]inter.TenantUser, error) {
	tenantID = bunrepo.NormalizeTenantID(tenantID)
	if _, err := r.GetTenant(tenantID); err != nil {
		return nil, err
	}

	var rows []bunrepo.TenantRoleRow
	if err := r.db.NewSelect().
		Model(&rows).
		Where("tenant_id = ?", tenantID).
		OrderExpr("created_at ASC, username ASC").
		Scan(context.Background()); err != nil {
		return nil, err
	}

	out := make([]inter.TenantUser, 0, len(rows))
	for _, row := range rows {
		out = append(out, tenantUserFromRow(row))
	}
	return out, nil
}

func (r *Repository) AddTenantUser(tenantID, username string, role inter.TenantRole) error {
	tenantID = bunrepo.NormalizeTenantID(tenantID)
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username is required")
	}
	if _, err := r.GetTenant(tenantID); err != nil {
		return err
	}
	return upsertTenantUser(context.Background(), r.db, tenantID, username, role, time.Now().UTC())
}

func upsertTenantUser(ctx context.Context, db bun.IDB, tenantID, username string, role inter.TenantRole, createdAt time.Time) error {
	row := &bunrepo.TenantRoleRow{
		TenantID:  tenantID,
		Username:  username,
		Role:      string(normalizeRole(role)),
		CreatedAt: createdAt,
	}
	_, err := db.NewRaw(`
		INSERT INTO tenant_users (tenant_id, username, role, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(tenant_id, username) DO UPDATE SET role = excluded.role
	`, row.TenantID, row.Username, row.Role, row.CreatedAt).Exec(ctx)
	return err
}

func (r *Repository) RemoveTenantUser(tenantID, username string) error {
	tenantID = bunrepo.NormalizeTenantID(tenantID)
	username = strings.TrimSpace(username)
	res, err := r.db.NewDelete().
		Model((*bunrepo.TenantRoleRow)(nil)).
		Where("tenant_id = ?", tenantID).
		Where("username = ?", username).
		Exec(context.Background())
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return inter.ErrTenantUserNotFound
	}
	return nil
}

func (r *Repository) CreateTenantInvitation(invitation inter.TenantInvitation) (inter.TenantInvitation, error) {
	tenantID := bunrepo.NormalizeTenantID(invitation.TenantID)
	username := strings.TrimSpace(invitation.Username)
	invitedBy := strings.TrimSpace(invitation.InvitedBy)

	if username == "" {
		return inter.TenantInvitation{}, errors.New("username is required")
	}
	if invitedBy == "" {
		return inter.TenantInvitation{}, errors.New("invited_by is required")
	}

	// 验证租户存在
	if _, err := r.GetTenant(tenantID); err != nil {
		return inter.TenantInvitation{}, err
	}

	id, err := newInvitationID()
	if err != nil {
		return inter.TenantInvitation{}, err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(7 * 24 * time.Hour) // 7天后过期

	row := &bunrepo.TenantInvitationRow{
		ID:        id,
		TenantID:  tenantID,
		Username:  username,
		Role:      string(normalizeRole(invitation.Role)),
		InvitedBy: invitedBy,
		Status:    "pending",
		ExpiresAt: expiresAt,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err := r.db.NewInsert().
		Model(row).
		Returning("NULL").
		Exec(context.Background()); err != nil {
		return inter.TenantInvitation{}, err
	}

	return invitationFromRow(*row), nil
}

func (r *Repository) ListPendingInvitations(username string) ([]inter.TenantInvitation, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return []inter.TenantInvitation{}, nil
	}

	var rows []bunrepo.TenantInvitationRow
	if err := r.db.NewSelect().
		Model(&rows).
		Where("username = ?", username).
		Where("status = ?", "pending").
		Where("expires_at > ?", time.Now().UTC()).
		OrderExpr("created_at DESC").
		Scan(context.Background()); err != nil {
		return nil, err
	}

	out := make([]inter.TenantInvitation, 0, len(rows))
	for _, row := range rows {
		out = append(out, invitationFromRow(row))
	}
	return out, nil
}

func (r *Repository) GetInvitation(invitationID string) (inter.TenantInvitation, error) {
	return getInvitation(context.Background(), r.db, invitationID)
}

func getInvitation(ctx context.Context, db bun.IDB, invitationID string) (inter.TenantInvitation, error) {
	var row bunrepo.TenantInvitationRow
	err := db.NewSelect().
		Model(&row).
		Where("id = ?", strings.TrimSpace(invitationID)).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return inter.TenantInvitation{}, inter.ErrInvitationNotFound
		}
		return inter.TenantInvitation{}, err
	}
	return invitationFromRow(row), nil
}

func (r *Repository) AcceptInvitation(invitationID string) error {
	invitationID = strings.TrimSpace(invitationID)
	return r.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		invitation, err := getInvitation(ctx, tx, invitationID)
		if err != nil {
			return err
		}
		if invitation.Status != "pending" {
			return inter.ErrInvitationAccepted
		}
		now := time.Now().UTC()
		if now.After(invitation.ExpiresAt) {
			return inter.ErrInvitationExpired
		}
		if err := upsertTenantUser(ctx, tx, invitation.TenantID, invitation.Username, invitation.Role, now); err != nil {
			return err
		}
		res, err := tx.NewUpdate().
			Model((*bunrepo.TenantInvitationRow)(nil)).
			Set("status = ?", "accepted").
			Set("updated_at = ?", now).
			Where("id = ?", invitationID).
			Where("status = ?", "pending").
			Where("expires_at > ?", now).
			Exec(ctx)
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return inter.ErrInvitationAccepted
		}
		return nil
	})
}

func (r *Repository) RejectInvitation(invitationID string) error {
	invitationID = strings.TrimSpace(invitationID)
	return r.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		invitation, err := getInvitation(ctx, tx, invitationID)
		if err != nil {
			return err
		}
		if invitation.Status != "pending" {
			return inter.ErrInvitationAccepted
		}
		now := time.Now().UTC()
		res, err := tx.NewUpdate().
			Model((*bunrepo.TenantInvitationRow)(nil)).
			Set("status = ?", "rejected").
			Set("updated_at = ?", now).
			Where("id = ?", invitationID).
			Where("status = ?", "pending").
			Exec(ctx)
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return inter.ErrInvitationAccepted
		}
		return nil
	})
}

func newInvitationID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "inv_" + hex.EncodeToString(b[:]), nil
}

func invitationFromRow(row bunrepo.TenantInvitationRow) inter.TenantInvitation {
	return inter.TenantInvitation{
		ID:        strings.TrimSpace(row.ID),
		TenantID:  strings.TrimSpace(row.TenantID),
		Username:  strings.TrimSpace(row.Username),
		Role:      normalizeRole(inter.TenantRole(row.Role)),
		InvitedBy: strings.TrimSpace(row.InvitedBy),
		Status:    strings.TrimSpace(row.Status),
		ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func tenantFromRow(row bunrepo.TenantRow) inter.Tenant {
	return inter.Tenant{
		ID:        strings.TrimSpace(row.ID),
		Name:      strings.TrimSpace(row.Name),
		Status:    normalizeTenantStatus(inter.TenantStatus(row.Status)),
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func tenantUserFromRow(row bunrepo.TenantRoleRow) inter.TenantUser {
	return inter.TenantUser{
		TenantID:  strings.TrimSpace(row.TenantID),
		Username:  strings.TrimSpace(row.Username),
		Role:      normalizeRole(inter.TenantRole(row.Role)),
		CreatedAt: row.CreatedAt,
	}
}

func normalizeTenantStatus(status inter.TenantStatus) inter.TenantStatus {
	switch strings.ToLower(strings.TrimSpace(string(status))) {
	case string(inter.TenantStatusSuspended):
		return inter.TenantStatusSuspended
	case string(inter.TenantStatusArchived):
		return inter.TenantStatusArchived
	default:
		return inter.TenantStatusActive
	}
}

func normalizeRole(role inter.TenantRole) inter.TenantRole {
	return bunrepo.NormalizeTenantRole(string(role))
}

func slugTenantID(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteRune('_')
		}
	}
	id := strings.Trim(b.String(), "_-")
	if id == "" {
		id = fmt.Sprintf("tenant_%d", time.Now().UnixNano())
	}
	if !strings.HasPrefix(id, "tenant_") {
		id = "tenant_" + id
	}
	return id
}
