package web

import (
	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	apiv1 "github.com/nhirsama/Goster-IoT/src/web/v1"
)

type loginAttemptGuard = apiv1.LoginAttemptGuard

func newLoginAttemptGuard(cfg appcfg.LoginProtectionConfig) *loginAttemptGuard {
	return apiv1.NewLoginAttemptGuard(cfg)
}

func (ws *webServer) loginAttemptProtector() *loginAttemptGuard {
	if ws.loginGuard == nil {
		ws.loginGuard = apiv1.NewLoginAttemptGuard(ws.webConfig().LoginProtection)
	}
	if ws.apiV1 != nil {
		ws.apiV1.SetLoginGuardForTest(ws.loginGuard)
	}
	return ws.loginGuard
}
