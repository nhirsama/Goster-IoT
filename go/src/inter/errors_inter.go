package inter

import "errors"

// 跨层共享的业务错误，供 Web/Service/Repository 使用 errors.Is 判断稳定语义。
var (
	ErrDeviceNotFound        = errors.New("device: not found")
	ErrDeviceAlreadyExists   = errors.New("device: already exists")
	ErrDeviceTokenNotFound   = errors.New("device: token not found")
	ErrUserNotFound          = errors.New("user: not found")
	ErrTenantNotFound        = errors.New("tenant: not found")
	ErrTenantUserNotFound    = errors.New("tenant user: not found")
	ErrDeviceTenantMismatch  = errors.New("device: tenant mismatch")
	ErrDeviceCommandNotFound = errors.New("device command: not found")
	ErrDownlinkQueueFull     = errors.New("downlink queue: full")
	ErrCannotRemoveSelf      = errors.New("tenant: cannot remove yourself")
	ErrInvitationNotFound    = errors.New("tenant invitation: not found")
	ErrInvitationExpired     = errors.New("tenant invitation: expired")
	ErrInvitationAccepted    = errors.New("tenant invitation: already processed")
)
