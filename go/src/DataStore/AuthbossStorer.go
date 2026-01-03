package DataStore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

// AuthUser implements authboss.User and related interfaces
type AuthUser struct {
	ID         int
	PID        string
	Email      string
	Username   string
	Password   string
	Permission int

	// Authboss modules
	RecoverToken       string
	RecoverTokenExpiry time.Time

	ConfirmToken string
	Confirmed    bool

	LastLogin time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GetPID returns the product ID (User's stable unique ID)
func (u *AuthUser) GetPID() string { return u.PID }

// PutPID sets the product ID
func (u *AuthUser) PutPID(pid string) { u.PID = pid }

// GetUsername returns the username (implements inter.SessionUser)
func (u *AuthUser) GetUsername() string { return u.Username }

// GetPermission returns the permission (implements inter.SessionUser)
func (u *AuthUser) GetPermission() inter.PermissionType { return inter.PermissionType(u.Permission) }

// GetPassword returns the password
func (u *AuthUser) GetPassword() string { return u.Password }

// PutPassword sets the password
func (u *AuthUser) PutPassword(password string) { u.Password = password }

// GetEmail returns the email
func (u *AuthUser) GetEmail() string { return u.Email }

// PutEmail sets the email
func (u *AuthUser) PutEmail(email string) { u.Email = email }

// GetConfirmed returns confirmed status
func (u *AuthUser) GetConfirmed() bool { return u.Confirmed }

// PutConfirmed sets confirmed status
func (u *AuthUser) PutConfirmed(confirmed bool) { u.Confirmed = confirmed }

// GetConfirmSelector returns confirm token
func (u *AuthUser) GetConfirmSelector() string { return u.ConfirmToken }

// PutConfirmSelector sets confirm token
func (u *AuthUser) PutConfirmSelector(token string) { u.ConfirmToken = token }

// GetRecoverSelector returns recover token
func (u *AuthUser) GetRecoverSelector() string { return u.RecoverToken }

// PutRecoverSelector sets recover token
func (u *AuthUser) PutRecoverSelector(token string) { u.RecoverToken = token }

// GetRecoverExpiry returns recover token expiry
func (u *AuthUser) GetRecoverExpiry() time.Time { return u.RecoverTokenExpiry }

// PutRecoverExpiry sets recover token expiry
func (u *AuthUser) PutRecoverExpiry(expiry time.Time) { u.RecoverTokenExpiry = expiry }

// --- Storer Implementation ---

// Load attempts to find a user by their PID (primary ID) OR Email.
// Authboss calls this with the Email during login, and with the PID during session validation.
func (s *DataStoreSql) Load(ctx context.Context, key string) (authboss.User, error) {
	user := &AuthUser{}

	// Handle nullable fields
	var recoverToken, confirmToken sql.NullString
	var recoverExpiry sql.NullTime
	var username sql.NullString

	query := `
		SELECT id, pid, email, username, password, permission, 
		       recover_token, recover_token_expiry, confirm_token, confirmed, created_at, updated_at
		FROM users 
		WHERE pid=? OR email=?`

	err := s.db.QueryRowContext(ctx, query, key, key).Scan(
		&user.ID, &user.PID, &user.Email, &username, &user.Password, &user.Permission,
		&recoverToken, &recoverExpiry, &confirmToken, &user.Confirmed, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, authboss.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	if username.Valid {
		user.Username = username.String
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

	return user, nil
}

// Save persists the user to the database
func (s *DataStoreSql) Save(ctx context.Context, user authboss.User) error {
	u, ok := user.(*AuthUser)
	if !ok {
		return errors.New("invalid user type")
	}

	// Prepare nullable fields
	var recoverToken, confirmToken sql.NullString
	var recoverExpiry sql.NullTime

	if u.RecoverToken != "" {
		recoverToken = sql.NullString{String: u.RecoverToken, Valid: true}
	}
	if u.RecoverTokenExpiry.IsZero() == false {
		recoverExpiry = sql.NullTime{Time: u.RecoverTokenExpiry, Valid: true}
	}
	if u.ConfirmToken != "" {
		confirmToken = sql.NullString{String: u.ConfirmToken, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET 
			email=?, password=?, permission=?, 
			recover_token=?, recover_token_expiry=?, 
			confirm_token=?, confirmed=?, updated_at=CURRENT_TIMESTAMP 
		WHERE pid=?`,
		u.Email, u.Password, u.Permission,
		recoverToken, recoverExpiry,
		confirmToken, u.Confirmed, u.PID,
	)
	return err
}

// New creates a blank user
func (s *DataStoreSql) New(ctx context.Context) authboss.User {
	return &AuthUser{}
}

// Create inserts a new user
func (s *DataStoreSql) Create(ctx context.Context, user authboss.User) error {
	u, ok := user.(*AuthUser)
	if !ok {
		return errors.New("invalid user type")
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (pid, email, username, password, permission, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		u.PID, u.Email, u.Username, u.Password, u.Permission,
	)
	return err
}
