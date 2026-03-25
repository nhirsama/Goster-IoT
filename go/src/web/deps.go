package web

import (
	"errors"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
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
	DataStore        inter.WebV1Store
	DeviceRegistry   inter.DeviceRegistry
	DevicePresence   inter.DevicePresence
	DownlinkCommands inter.DownlinkCommandService
	Auth             AuthService
	Captcha          CaptchaVerifier
	Logger           inter.Logger
	Config           appcfg.WebConfig
}

func (d *WebServerDeps) normalize() error {
	if d.DataStore == nil {
		return errors.New("web deps missing datastore")
	}
	if d.DeviceRegistry == nil {
		return errors.New("web deps missing device registry")
	}
	if d.DevicePresence == nil {
		return errors.New("web deps missing device presence")
	}
	if d.DownlinkCommands == nil {
		return errors.New("web deps missing downlink command service")
	}
	if d.Auth == nil {
		return errors.New("web deps missing auth service")
	}
	if d.Captcha == nil {
		d.Captcha = NewTurnstileServiceWithConfig(appcfg.DefaultCaptchaConfig())
	}
	if d.Logger == nil {
		d.Logger = logger.Default()
	}
	d.Config = appcfg.NormalizeWebConfig(d.Config)
	return nil
}
