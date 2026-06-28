package identity

import (
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// AuthUser 是认证链路使用的用户模型。
// 它只承载身份凭据与会话相关字段，不负责租户授权决策。
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
