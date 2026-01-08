package datastore

import (
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// AuthUser 实现了 authboss.User 及相关接口
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

// GetPID 获取用户唯一标识符 (Username)
func (u *AuthUser) GetPID() string { return u.Username }

// PutPID 设置用户唯一标识符 (Username)
func (u *AuthUser) PutPID(pid string) { u.Username = pid }

// GetUsername 获取用户名
func (u *AuthUser) GetUsername() string { return u.Username }

// PutUsername 设置用户名
func (u *AuthUser) PutUsername(username string) { u.Username = username }

// GetPermission 获取用户权限 (实现 inter.SessionUser)
func (u *AuthUser) GetPermission() inter.PermissionType { return inter.PermissionType(u.Permission) }

// GetPassword 获取密码哈希
func (u *AuthUser) GetPassword() string { return u.Password }

// PutPassword 设置密码哈希
func (u *AuthUser) PutPassword(password string) { u.Password = password }

// GetEmail 获取邮箱
func (u *AuthUser) GetEmail() string { return u.Email }

// PutEmail 设置邮箱
func (u *AuthUser) PutEmail(email string) { u.Email = email }

// GetConfirmed 获取邮箱验证状态
func (u *AuthUser) GetConfirmed() bool { return u.Confirmed }

// PutConfirmed 设置邮箱验证状态
func (u *AuthUser) PutConfirmed(confirmed bool) { u.Confirmed = confirmed }

// GetConfirmSelector 获取验证 Token
func (u *AuthUser) GetConfirmSelector() string { return u.ConfirmToken }

// PutConfirmSelector 设置验证 Token
func (u *AuthUser) PutConfirmSelector(token string) { u.ConfirmToken = token }

// GetRecoverSelector 获取找回密码 Token
func (u *AuthUser) GetRecoverSelector() string { return u.RecoverToken }

// PutRecoverSelector 设置找回密码 Token
func (u *AuthUser) PutRecoverSelector(token string) { u.RecoverToken = token }

// GetRecoverExpiry 获取找回密码 Token 过期时间
func (u *AuthUser) GetRecoverExpiry() time.Time { return u.RecoverTokenExpiry }

// PutRecoverExpiry 设置找回密码 Token 过期时间
func (u *AuthUser) PutRecoverExpiry(expiry time.Time) { u.RecoverTokenExpiry = expiry }

// GetOAuth2UID 获取 OAuth2 UID
func (u *AuthUser) GetOAuth2UID() string { return u.OAuth2UID }

// PutOAuth2UID 设置 OAuth2 UID
func (u *AuthUser) PutOAuth2UID(uid string) { u.OAuth2UID = uid }

// GetOAuth2Provider 获取 OAuth2 提供商
func (u *AuthUser) GetOAuth2Provider() string { return u.OAuth2Provider }

// PutOAuth2Provider 设置 OAuth2 提供商
func (u *AuthUser) PutOAuth2Provider(provider string) { u.OAuth2Provider = provider }

// GetOAuth2AccessToken 获取 OAuth2 Access Token
func (u *AuthUser) GetOAuth2AccessToken() string { return u.OAuth2AccessToken }

// PutOAuth2AccessToken 设置 OAuth2 Access Token
func (u *AuthUser) PutOAuth2AccessToken(token string) { u.OAuth2AccessToken = token }

// GetOAuth2RefreshToken 获取 OAuth2 Refresh Token
func (u *AuthUser) GetOAuth2RefreshToken() string { return u.OAuth2RefreshToken }

// PutOAuth2RefreshToken 设置 OAuth2 Refresh Token
func (u *AuthUser) PutOAuth2RefreshToken(token string) { u.OAuth2RefreshToken = token }

// GetOAuth2Expiry 获取 OAuth2 Token 过期时间
func (u *AuthUser) GetOAuth2Expiry() time.Time { return u.OAuth2Expiry }

// PutOAuth2Expiry 设置 OAuth2 Token 过期时间
func (u *AuthUser) PutOAuth2Expiry(expiry time.Time) { u.OAuth2Expiry = expiry }

// GetRememberToken 获取记住我 Token
func (u *AuthUser) GetRememberToken() string { return u.RememberToken }

// PutRememberToken 设置记住我 Token
func (u *AuthUser) PutRememberToken(token string) { u.RememberToken = token }

// IsOAuth2User 判断是否为 OAuth2 用户
func (u *AuthUser) IsOAuth2User() bool { return u.OAuth2UID != "" }
