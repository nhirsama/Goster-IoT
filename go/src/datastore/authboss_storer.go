package datastore

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

// Load 尝试通过 PID (Username) 或 Authboss OAuth2 PID 查找用户
func (s *DataStoreSql) Load(ctx context.Context, key string) (authboss.User, error) {
	user := &AuthUser{}
	var recoverToken, confirmToken, oauth2Uid, oauth2Provider, oauth2Access, oauth2Refresh, rememberToken sql.NullString
	var recoverExpiry, oauth2Expiry sql.NullTime
	var email sql.NullString

	var query string
	var args []interface{}

	// 处理 Authboss OAuth2 PID 格式: "oauth2;;provider;;uid"
	if strings.HasPrefix(key, "oauth2;;") {
		parts := strings.Split(key, ";;")
		if len(parts) == 3 {
			provider, uid := parts[1], parts[2]
			query = `
				SELECT id, email, username, password, permission, 
				       recover_token, recover_token_expiry, confirm_token, confirmed, created_at, updated_at,
				       oauth2_uid, oauth2_provider, oauth2_access_token, oauth2_refresh_token, oauth2_expiry, remember_token
				FROM users 
				WHERE oauth2_provider=? AND oauth2_uid=?`
			args = []interface{}{provider, uid}
		} else {
			return nil, authboss.ErrUserNotFound
		}
	} else {
		// 标准 Username 查找
		query = `
			SELECT id, email, username, password, permission, 
			       recover_token, recover_token_expiry, confirm_token, confirmed, created_at, updated_at,
			       oauth2_uid, oauth2_provider, oauth2_access_token, oauth2_refresh_token, oauth2_expiry, remember_token
			FROM users 
			WHERE username=?`
		args = []interface{}{key}
	}

	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&user.ID, &email, &user.Username, &user.Password, &user.Permission,
		&recoverToken, &recoverExpiry, &confirmToken, &user.Confirmed, &user.CreatedAt, &user.UpdatedAt,
		&oauth2Uid, &oauth2Provider, &oauth2Access, &oauth2Refresh, &oauth2Expiry, &rememberToken,
	)

	if err == sql.ErrNoRows {
		return nil, authboss.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	if email.Valid {
		user.Email = email.String
	}
	if recoverToken.Valid {
		user.RecoverToken = recoverToken.String
	}
	if recoverExpiry.Valid {
		user.RecoverTokenExpiry = recoverExpiry.Time
	}
	if confirmToken.Valid {
		user.ConfirmToken = confirmToken.String
	}
	if oauth2Uid.Valid {
		user.OAuth2UID = oauth2Uid.String
	}
	if oauth2Provider.Valid {
		user.OAuth2Provider = oauth2Provider.String
	}
	if oauth2Access.Valid {
		user.OAuth2AccessToken = oauth2Access.String
	}
	if oauth2Refresh.Valid {
		user.OAuth2RefreshToken = oauth2Refresh.String
	}
	if oauth2Expiry.Valid {
		user.OAuth2Expiry = oauth2Expiry.Time
	}
	if rememberToken.Valid {
		user.RememberToken = rememberToken.String
	}

	return user, nil
}

// Save 持久化用户数据到数据库
func (s *DataStoreSql) Save(ctx context.Context, user authboss.User) error {
	u, ok := user.(*AuthUser)
	if !ok {
		return errors.New("save: 无效的用户类型")
	}

	var recoverToken, confirmToken, oauth2Uid, oauth2Provider, oauth2Access, oauth2Refresh, rememberToken sql.NullString
	var recoverExpiry, oauth2Expiry sql.NullTime

	if u.RecoverToken != "" {
		recoverToken = sql.NullString{String: u.RecoverToken, Valid: true}
	}
	if u.RecoverTokenExpiry.IsZero() == false {
		recoverExpiry = sql.NullTime{Time: u.RecoverTokenExpiry, Valid: true}
	}
	if u.ConfirmToken != "" {
		confirmToken = sql.NullString{String: u.ConfirmToken, Valid: true}
	}
	if u.OAuth2UID != "" {
		oauth2Uid = sql.NullString{String: u.OAuth2UID, Valid: true}
	}
	if u.OAuth2Provider != "" {
		oauth2Provider = sql.NullString{String: u.OAuth2Provider, Valid: true}
	}
	if u.OAuth2AccessToken != "" {
		oauth2Access = sql.NullString{String: u.OAuth2AccessToken, Valid: true}
	}
	if u.OAuth2RefreshToken != "" {
		oauth2Refresh = sql.NullString{String: u.OAuth2RefreshToken, Valid: true}
	}
	if u.OAuth2Expiry.IsZero() == false {
		oauth2Expiry = sql.NullTime{Time: u.OAuth2Expiry, Valid: true}
	}
	if u.RememberToken != "" {
		rememberToken = sql.NullString{String: u.RememberToken, Valid: true}
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE users SET 
			email=?, password=?, permission=?, 
			recover_token=?, recover_token_expiry=?, 
			confirm_token=?, confirmed=?, 
			oauth2_uid=?, oauth2_provider=?, oauth2_access_token=?, oauth2_refresh_token=?, oauth2_expiry=?,
			remember_token=?, updated_at=CURRENT_TIMESTAMP 
		WHERE username=?`,
		u.Email, u.Password, u.Permission,
		recoverToken, recoverExpiry,
		confirmToken, u.Confirmed,
		oauth2Uid, oauth2Provider, oauth2Access, oauth2Refresh, oauth2Expiry,
		rememberToken, u.Username,
	)
	if err != nil {
		return err
	}

	// 如果 UPDATE 影响行数为 0 (用户不存在), 回退到 Create
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return s.Create(ctx, u)
	}

	return nil
}

// New 创建一个空白用户
func (s *DataStoreSql) New(ctx context.Context) authboss.User {
	return &AuthUser{}
}

// Create 插入新用户
func (s *DataStoreSql) Create(ctx context.Context, user authboss.User) error {
	u, ok := user.(*AuthUser)
	if !ok {
		return errors.New("create: 无效的用户类型")
	}

	// 检查是否为第一个用户 (如果是则设为管理员)
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		u.Permission = int(inter.PermissionAdmin)
	} else {
		u.Permission = int(inter.PermissionNone)
	}

	// 处理 OAuth2 注册: 自动生成用户名
	if u.Username == "" && u.OAuth2Provider != "" {
		u.Username = u.OAuth2Provider + "_" + u.OAuth2UID
	}

	// 如果密码为空 (OAuth2), 生成占位密码以满足 NOT NULL 约束
	if u.Password == "" {
		u.Password = "oauth2_dummy_password_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO users (email, username, password, permission, 
			oauth2_uid, oauth2_provider, oauth2_access_token, oauth2_refresh_token, oauth2_expiry,
			created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		u.Email, u.Username, u.Password, u.Permission,
		u.OAuth2UID, u.OAuth2Provider, u.OAuth2AccessToken, u.OAuth2RefreshToken, u.OAuth2Expiry,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return authboss.ErrUserFound
		}
		return err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	u.ID = int(id)

	return nil
}

// NewFromOAuth2 根据 OAuth2 详情创建用户
func (s *DataStoreSql) NewFromOAuth2(ctx context.Context, provider string, details map[string]string) (authboss.OAuth2User, error) {
	username := provider + "_" + details["uid"]

	// 尝试加载现有用户以保留权限等字段
	existingUser, err := s.Load(ctx, username)
	if err == nil {
		if u, ok := existingUser.(*AuthUser); ok {
			u.OAuth2UID = details["uid"]
			u.OAuth2Provider = provider
			u.Email = details["email"]
			return u, nil
		}
	}

	u := &AuthUser{
		OAuth2Provider: provider,
		OAuth2UID:      details["uid"],
		Email:          details["email"],
		// 关键: 必须在此处构造 Username (PID) 以便 Authboss 能够通过 Load() 找到该用户。
		// 此逻辑需与 Create() 中的生成逻辑保持一致。
		Username: username,
	}
	return u, nil
}

// SaveOAuth2 保存 OAuth2 用户
func (s *DataStoreSql) SaveOAuth2(ctx context.Context, user authboss.OAuth2User) error {
	u, ok := user.(*AuthUser)
	if !ok {
		return errors.New("saveOAuth2: 无效的用户类型")
	}
	return s.Save(ctx, u)
}

// LoadByRememberToken 通过记住我 Token 查找用户
func (s *DataStoreSql) LoadByRememberToken(ctx context.Context, token string) (authboss.User, error) {
	user := &AuthUser{}
	var recoverToken, confirmToken, oauth2Uid, oauth2Provider, oauth2Access, oauth2Refresh, rememberToken sql.NullString
	var recoverExpiry, oauth2Expiry sql.NullTime
	var email sql.NullString

	query := `
		SELECT id, email, username, password, permission, 
		       recover_token, recover_token_expiry, confirm_token, confirmed, created_at, updated_at,
		       oauth2_uid, oauth2_provider, oauth2_access_token, oauth2_refresh_token, oauth2_expiry, remember_token
		FROM users 
		WHERE remember_token=?`

	err := s.db.QueryRowContext(ctx, query, token).Scan(
		&user.ID, &email, &user.Username, &user.Password, &user.Permission,
		&recoverToken, &recoverExpiry, &confirmToken, &user.Confirmed, &user.CreatedAt, &user.UpdatedAt,
		&oauth2Uid, &oauth2Provider, &oauth2Access, &oauth2Refresh, &oauth2Expiry, &rememberToken,
	)

	if err == sql.ErrNoRows {
		return nil, authboss.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	if email.Valid {
		user.Email = email.String
	}
	if recoverToken.Valid {
		user.RecoverToken = recoverToken.String
	}
	if recoverExpiry.Valid {
		user.RecoverTokenExpiry = recoverExpiry.Time
	}
	if confirmToken.Valid {
		user.ConfirmToken = confirmToken.String
	}
	if oauth2Uid.Valid {
		user.OAuth2UID = oauth2Uid.String
	}
	if oauth2Provider.Valid {
		user.OAuth2Provider = oauth2Provider.String
	}
	if oauth2Access.Valid {
		user.OAuth2AccessToken = oauth2Access.String
	}
	if oauth2Refresh.Valid {
		user.OAuth2RefreshToken = oauth2Refresh.String
	}
	if oauth2Expiry.Valid {
		user.OAuth2Expiry = oauth2Expiry.Time
	}
	if rememberToken.Valid {
		user.RememberToken = rememberToken.String
	}

	return user, nil
}
