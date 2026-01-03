package DataStore

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

// AuthUser implements authboss.User and related interfaces
type AuthUser struct {
	ID                 int
	Email              string
	Username           string
	Password           string
	Permission         int
	RecoverToken       string
	RecoverTokenExpiry time.Time
	ConfirmToken       string
	Confirmed          bool
	LastLogin          time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	OAuth2UID          string
	OAuth2Provider     string
	OAuth2AccessToken  string
	OAuth2RefreshToken string
	OAuth2Expiry       time.Time
	RememberToken      string
}

func (u *AuthUser) GetPID() string                      { return u.Username }
func (u *AuthUser) PutPID(pid string)                   { u.Username = pid }
func (u *AuthUser) GetUsername() string                 { return u.Username }
func (u *AuthUser) PutUsername(username string)         { u.Username = username }
func (u *AuthUser) GetPermission() inter.PermissionType { return inter.PermissionType(u.Permission) }
func (u *AuthUser) GetPassword() string                 { return u.Password }
func (u *AuthUser) PutPassword(password string)         { u.Password = password }
func (u *AuthUser) GetEmail() string                    { return u.Email }
func (u *AuthUser) PutEmail(email string)               { u.Email = email }
func (u *AuthUser) GetConfirmed() bool                  { return u.Confirmed }
func (u *AuthUser) PutConfirmed(confirmed bool)         { u.Confirmed = confirmed }
func (u *AuthUser) GetConfirmSelector() string          { return u.ConfirmToken }
func (u *AuthUser) PutConfirmSelector(token string)     { u.ConfirmToken = token }
func (u *AuthUser) GetRecoverSelector() string          { return u.RecoverToken }
func (u *AuthUser) PutRecoverSelector(token string)     { u.RecoverToken = token }
func (u *AuthUser) GetRecoverExpiry() time.Time         { return u.RecoverTokenExpiry }
func (u *AuthUser) PutRecoverExpiry(expiry time.Time)   { u.RecoverTokenExpiry = expiry }
func (u *AuthUser) GetOAuth2UID() string                { return u.OAuth2UID }
func (u *AuthUser) PutOAuth2UID(uid string)             { u.OAuth2UID = uid }
func (u *AuthUser) GetOAuth2Provider() string           { return u.OAuth2Provider }
func (u *AuthUser) PutOAuth2Provider(provider string)   { u.OAuth2Provider = provider }
func (u *AuthUser) GetOAuth2AccessToken() string        { return u.OAuth2AccessToken }
func (u *AuthUser) PutOAuth2AccessToken(token string)   { u.OAuth2AccessToken = token }
func (u *AuthUser) GetOAuth2RefreshToken() string       { return u.OAuth2RefreshToken }
func (u *AuthUser) PutOAuth2RefreshToken(token string)  { u.OAuth2RefreshToken = token }
func (u *AuthUser) GetOAuth2Expiry() time.Time          { return u.OAuth2Expiry }
func (u *AuthUser) PutOAuth2Expiry(expiry time.Time)    { u.OAuth2Expiry = expiry }
func (u *AuthUser) GetRememberToken() string            { return u.RememberToken }
func (u *AuthUser) PutRememberToken(token string)       { u.RememberToken = token }
func (u *AuthUser) IsOAuth2User() bool                  { return u.OAuth2UID != "" }

// Load attempts to find a user by their PID (Username) or Authboss OAuth2 PID.
func (s *DataStoreSql) Load(ctx context.Context, key string) (authboss.User, error) {
	log.Printf("Load called with key: %s", key)
	user := &AuthUser{}
	var recoverToken, confirmToken, oauth2Uid, oauth2Provider, oauth2Access, oauth2Refresh, rememberToken sql.NullString
	var recoverExpiry, oauth2Expiry sql.NullTime
	var email sql.NullString

	var query string
	var args []interface{}

	// Handle Authboss OAuth2 PID format: "oauth2;;provider;;uid"
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
		// Standard Username lookup
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
		log.Printf("Load: User not found for key: %s", key)
		return nil, authboss.ErrUserNotFound
	}
	if err != nil {
		log.Printf("Load: DB Error: %v", err)
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

	log.Printf("Load: Found user: %s", user.Username)
	return user, nil
}

// Save persists the user to the database
func (s *DataStoreSql) Save(ctx context.Context, user authboss.User) error {
	u, ok := user.(*AuthUser)
	if !ok {
		return errors.New("invalid user type")
	}
	log.Printf("Save called for user: %s", u.Username)

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
		log.Printf("Save DB Error: %v", err)
		return err
	}

	// Fallback to Create if UPDATE affected 0 rows (User doesn't exist)
	rows, _ := res.RowsAffected()
	if rows == 0 {
		log.Printf("Save: No rows affected, falling back to Create for %s", u.Username)
		return s.Create(ctx, u)
	}

	return nil
}

func (s *DataStoreSql) New(ctx context.Context) authboss.User {
	return &AuthUser{}
}

// Create inserts a new user
func (s *DataStoreSql) Create(ctx context.Context, user authboss.User) error {
	u, ok := user.(*AuthUser)
	if !ok {
		return errors.New("invalid user type")
	}
	log.Printf("Create called. Username: %s, Email: %s, Provider: %s", u.Username, u.Email, u.OAuth2Provider)

	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		log.Printf("Create: Count Error: %v", err)
		return err
	}

	if count == 0 {
		u.Permission = int(inter.PermissionAdmin)
	} else {
		u.Permission = int(inter.PermissionNone)
	}

	// For OAuth2 registration:
	if u.Username == "" && u.OAuth2Provider != "" {
		u.Username = u.OAuth2Provider + "_" + u.OAuth2UID
		log.Printf("Create: Generated Username: %s", u.Username)
	}

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
		log.Printf("Create DB Insert Error: %v", err)
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
	log.Printf("Create: Success, ID=%d", u.ID)

	return nil
}

// NewFromOAuth2 creates a user from OAuth2 details
func (s *DataStoreSql) NewFromOAuth2(ctx context.Context, provider string, details map[string]string) (authboss.OAuth2User, error) {
	log.Printf("NewFromOAuth2 called. Provider: %s, UID: %s", provider, details["uid"])
	u := &AuthUser{
		OAuth2Provider: provider,
		OAuth2UID:      details["uid"],
		Email:          details["email"],
		// CRITICAL: We must construct the Username (PID) here so Authboss can Load() the user.
		Username: provider + "_" + details["uid"],
	}
	return u, nil
}

// SaveOAuth2 saves the oauth2 user
func (s *DataStoreSql) SaveOAuth2(ctx context.Context, user authboss.OAuth2User) error {
	log.Printf("SaveOAuth2 called")
	u, ok := user.(*AuthUser)
	if !ok {
		return errors.New("invalid user type in SaveOAuth2")
	}
	return s.Save(ctx, u)
}

// LoadByRememberToken finds a user by their remember token
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
