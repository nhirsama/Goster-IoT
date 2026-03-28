package inter

import "errors"

// 跨层共享的业务错误，供 Web/Service/Repository 使用 errors.Is 判断稳定语义。
var (
	ErrDeviceNotFound        = errors.New("device: not found")
	ErrDeviceTokenNotFound   = errors.New("device: token not found")
	ErrUserNotFound          = errors.New("user: not found")
	ErrDeviceTenantMismatch  = errors.New("device: tenant mismatch")
	ErrDeviceCommandNotFound = errors.New("device command: not found")
	ErrDownlinkQueueFull     = errors.New("downlink queue: full")
)
