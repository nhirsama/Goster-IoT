package identitystore

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/aarondl/authboss/v3"
	identitycore "github.com/nhirsama/Goster-IoT/src/identity"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/bunrepo"
	userstorage "github.com/nhirsama/Goster-IoT/src/storage/user"
	"github.com/uptrace/bun"
)

// Repository 是认证链路使用的独立存储模块。
// 它只负责用户凭据、OAuth2 信息和 remember token，不承担业务查询职责。
type Repository struct {
	base *bunrepo.Store
	db   *bun.DB
}

var _ identitycore.Store = (*Repository)(nil)

func OpenSQLite(path string) (*Repository, error) {
	base, err := bunrepo.OpenSQLite(path)
	if err != nil {
		return nil, err
	}
	return &Repository{base: base, db: base.DB}, nil
}

func OpenPostgres(dsn string) (*Repository, error) {
	base, err := bunrepo.OpenPostgres(dsn)
	if err != nil {
		return nil, err
	}
	return &Repository{base: base, db: base.DB}, nil
}

func (r *Repository) Close() error {
	if r == nil || r.base == nil {
		return nil
	}
	return r.base.Close()
}

func (r *Repository) Load(ctx context.Context, key string) (authboss.User, error) {
	row := new(authUserRow)
	query := r.db.NewSelect().Model(row).Limit(1)

	if strings.HasPrefix(key, "oauth2;;") {
		parts := strings.Split(key, ";;")
		if len(parts) != 3 {
			return nil, authboss.ErrUserNotFound
		}
		query = query.Where("oauth2_provider = ?", parts[1]).Where("oauth2_uid = ?", parts[2])
	} else {
		query = query.Where("username = ?", strings.TrimSpace(key))
	}

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, authboss.ErrUserNotFound
		}
		return nil, err
	}
	return row.toAuthUser(), nil
}

func (r *Repository) Save(ctx context.Context, user authboss.User) error {
	u, ok := user.(*identitycore.AuthUser)
	if !ok {
		return errors.New("save: 无效的认证用户类型")
	}

	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewUpdate().
			Model((*authUserRow)(nil)).
			Set("email = ?", nullableString(u.Email)).
			Set("password = ?", u.Password).
			Set("permission = ?", u.Permission).
			Set("recover_token = ?", nullableString(u.RecoverToken)).
			Set("recover_token_expiry = ?", nullableTime(u.RecoverTokenExpiry)).
			Set("confirm_token = ?", nullableString(u.ConfirmToken)).
			Set("confirmed = ?", u.Confirmed).
			Set("oauth2_uid = ?", nullableString(u.OAuth2UID)).
			Set("oauth2_provider = ?", nullableString(u.OAuth2Provider)).
			Set("oauth2_access_token = ?", nullableString(u.OAuth2AccessToken)).
			Set("oauth2_refresh_token = ?", nullableString(u.OAuth2RefreshToken)).
			Set("oauth2_expiry = ?", nullableTime(u.OAuth2Expiry)).
			Set("remember_token = ?", nullableString(u.RememberToken)).
			Set("updated_at = CURRENT_TIMESTAMP").
			Where("username = ?", strings.TrimSpace(u.Username)).
			Returning("NULL").
			Exec(ctx)
		if err != nil {
			return err
		}

		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return authboss.ErrUserNotFound
		}

		return userstorage.SyncLegacyTenantRole(ctx, tx, u.Username, inter.PermissionType(u.Permission))
	})
}

func (r *Repository) New(ctx context.Context) authboss.User {
	return &identitycore.AuthUser{}
}

func (r *Repository) Create(ctx context.Context, user authboss.User) error {
	u, ok := user.(*identitycore.AuthUser)
	if !ok {
		return errors.New("create: 无效的认证用户类型")
	}

	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var count int
		if err := tx.NewSelect().Table("users").ColumnExpr("COUNT(*)").Scan(ctx, &count); err != nil {
			return err
		}

		if count == 0 {
			u.Permission = int(inter.PermissionAdmin)
		} else {
			u.Permission = int(inter.PermissionNone)
		}
		if strings.TrimSpace(u.Username) == "" && strings.TrimSpace(u.OAuth2Provider) != "" {
			u.Username = u.OAuth2Provider + "_" + u.OAuth2UID
		}
		if strings.TrimSpace(u.Password) == "" {
			u.Password = "oauth2_dummy_password_" + strconv.FormatInt(time.Now().UnixNano(), 10)
		}

		row := newAuthUserRow(u)
		if _, err := tx.NewInsert().Model(row).Exec(ctx); err != nil {
			if isUniqueConstraint(err) {
				return authboss.ErrUserFound
			}
			return err
		}
		u.ID = int(row.ID)

		return userstorage.SyncLegacyTenantRole(ctx, tx, u.Username, inter.PermissionType(u.Permission))
	})
}

func (r *Repository) NewFromOAuth2(ctx context.Context, provider string, details map[string]string) (authboss.OAuth2User, error) {
	username := provider + "_" + details["uid"]

	existingUser, err := r.Load(ctx, username)
	if err == nil {
		if u, ok := existingUser.(*identitycore.AuthUser); ok {
			u.OAuth2UID = details["uid"]
			u.OAuth2Provider = provider
			u.Email = details["email"]
			return u, nil
		}
	}

	return &identitycore.AuthUser{
		Username:       username,
		OAuth2Provider: provider,
		OAuth2UID:      details["uid"],
		Email:          details["email"],
	}, nil
}

func (r *Repository) SaveOAuth2(ctx context.Context, user authboss.OAuth2User) error {
	u, ok := user.(*identitycore.AuthUser)
	if !ok {
		return errors.New("saveOAuth2: 无效的认证用户类型")
	}
	if err := r.Save(ctx, u); err != nil {
		if errors.Is(err, authboss.ErrUserNotFound) {
			return r.Create(ctx, u)
		}
		return err
	}
	return nil
}

func (r *Repository) AddRememberToken(ctx context.Context, pid, token string) error {
	res, err := r.db.NewUpdate().
		Table("users").
		Set("remember_token = ?", token).
		Set("updated_at = CURRENT_TIMESTAMP").
		Where("username = ?", strings.TrimSpace(pid)).
		Returning("NULL").
		Exec(ctx)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return authboss.ErrUserNotFound
	}
	return nil
}

func (r *Repository) DelRememberTokens(ctx context.Context, pid string) error {
	_, err := r.db.NewUpdate().
		Table("users").
		Set("remember_token = NULL").
		Set("updated_at = CURRENT_TIMESTAMP").
		Where("username = ?", strings.TrimSpace(pid)).
		Returning("NULL").
		Exec(ctx)
	return err
}

func (r *Repository) UseRememberToken(ctx context.Context, pid, token string) error {
	res, err := r.db.NewUpdate().
		Table("users").
		Set("remember_token = NULL").
		Set("updated_at = CURRENT_TIMESTAMP").
		Where("username = ?", strings.TrimSpace(pid)).
		Where("remember_token = ?", token).
		Returning("NULL").
		Exec(ctx)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return authboss.ErrTokenNotFound
	}
	return nil
}

type authUserRow struct {
	bun.BaseModel `bun:"table:users"`

	ID                 int64          `bun:"id,pk,autoincrement"`
	Email              sql.NullString `bun:"email"`
	Username           string         `bun:"username"`
	Password           string         `bun:"password"`
	Permission         int            `bun:"permission"`
	OAuth2UID          sql.NullString `bun:"oauth2_uid"`
	OAuth2Provider     sql.NullString `bun:"oauth2_provider"`
	OAuth2AccessToken  sql.NullString `bun:"oauth2_access_token"`
	OAuth2RefreshToken sql.NullString `bun:"oauth2_refresh_token"`
	OAuth2Expiry       sql.NullTime   `bun:"oauth2_expiry"`
	RememberToken      sql.NullString `bun:"remember_token"`
	RecoverToken       sql.NullString `bun:"recover_token"`
	RecoverTokenExpiry sql.NullTime   `bun:"recover_token_expiry"`
	ConfirmToken       sql.NullString `bun:"confirm_token"`
	Confirmed          bool           `bun:"confirmed"`
	LastLogin          sql.NullTime   `bun:"last_login"`
	CreatedAt          time.Time      `bun:"created_at"`
	UpdatedAt          time.Time      `bun:"updated_at"`
}

func newAuthUserRow(u *identitycore.AuthUser) *authUserRow {
	return &authUserRow{
		ID:                 int64(u.ID),
		Email:              nullableString(u.Email),
		Username:           strings.TrimSpace(u.Username),
		Password:           u.Password,
		Permission:         u.Permission,
		OAuth2UID:          nullableString(u.OAuth2UID),
		OAuth2Provider:     nullableString(u.OAuth2Provider),
		OAuth2AccessToken:  nullableString(u.OAuth2AccessToken),
		OAuth2RefreshToken: nullableString(u.OAuth2RefreshToken),
		OAuth2Expiry:       nullableTime(u.OAuth2Expiry),
		RememberToken:      nullableString(u.RememberToken),
		RecoverToken:       nullableString(u.RecoverToken),
		RecoverTokenExpiry: nullableTime(u.RecoverTokenExpiry),
		ConfirmToken:       nullableString(u.ConfirmToken),
		Confirmed:          u.Confirmed,
		LastLogin:          nullableTime(u.LastLogin),
	}
}

func (r *authUserRow) toAuthUser() *identitycore.AuthUser {
	out := &identitycore.AuthUser{
		ID:         int(r.ID),
		Username:   r.Username,
		Password:   r.Password,
		Permission: r.Permission,
		Confirmed:  r.Confirmed,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
	if r.Email.Valid {
		out.Email = r.Email.String
	}
	if r.OAuth2UID.Valid {
		out.OAuth2UID = r.OAuth2UID.String
	}
	if r.OAuth2Provider.Valid {
		out.OAuth2Provider = r.OAuth2Provider.String
	}
	if r.OAuth2AccessToken.Valid {
		out.OAuth2AccessToken = r.OAuth2AccessToken.String
	}
	if r.OAuth2RefreshToken.Valid {
		out.OAuth2RefreshToken = r.OAuth2RefreshToken.String
	}
	if r.OAuth2Expiry.Valid {
		out.OAuth2Expiry = r.OAuth2Expiry.Time
	}
	if r.RememberToken.Valid {
		out.RememberToken = r.RememberToken.String
	}
	if r.RecoverToken.Valid {
		out.RecoverToken = r.RecoverToken.String
	}
	if r.RecoverTokenExpiry.Valid {
		out.RecoverTokenExpiry = r.RecoverTokenExpiry.Time
	}
	if r.ConfirmToken.Valid {
		out.ConfirmToken = r.ConfirmToken.String
	}
	if r.LastLogin.Valid {
		out.LastLogin = r.LastLogin.Time
	}
	return out
}

func nullableString(v string) sql.NullString {
	v = strings.TrimSpace(v)
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func nullableTime(v time.Time) sql.NullTime {
	if v.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: v, Valid: true}
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}
