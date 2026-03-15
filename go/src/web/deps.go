package web

import (
	"errors"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
)

// CaptchaVerifier 抽象验证码服务，便于注入不同实现与测试替身。
type CaptchaVerifier interface {
	IsEnabled() bool
	PublicSiteKey() string
	VerifyToken(token string, ip string) bool
}

// WebServerDeps 描述 web 模块运行所需依赖。
type WebServerDeps struct {
	DataStore     inter.DataStore
	DeviceManager inter.DeviceManager
	API           inter.Api
	Auth          AuthService
	Captcha       CaptchaVerifier
	Logger        inter.Logger
}

func (d *WebServerDeps) normalize() error {
	if d.DataStore == nil {
		return errors.New("web deps missing datastore")
	}
	if d.DeviceManager == nil {
		return errors.New("web deps missing device manager")
	}
	if d.API == nil {
		return errors.New("web deps missing api service")
	}
	if d.Auth == nil {
		return errors.New("web deps missing auth service")
	}
	if d.Captcha == nil {
		d.Captcha = NewTurnstileService()
	}
	if d.Logger == nil {
		d.Logger = logger.Default()
	}
	return nil
}
