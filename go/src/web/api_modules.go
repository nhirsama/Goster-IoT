package web

import (
	"net/http"

	apiv1 "github.com/nhirsama/Goster-IoT/src/web/v1"
)

// apiModule 代表一个自包含的 API 版本模块。
// 根 web 包只负责编排模块，不直接依赖具体版本的 handler 细节。
type apiModule interface {
	RegisterRoutes(mux *http.ServeMux)
}

type apiModuleFactory func(deps WebServerDeps) apiModule

func defaultAPIModuleFactories() []apiModuleFactory {
	return []apiModuleFactory{
		newAPIV1Module,
	}
}

// buildAPIModules 根据装配配置实例化所有启用中的 API 版本模块。
func buildAPIModules(deps WebServerDeps) []apiModule {
	factories := defaultAPIModuleFactories()
	modules := make([]apiModule, 0, len(factories))
	for _, factory := range factories {
		modules = append(modules, factory(deps))
	}
	return modules
}

// newAPIV1Module 负责把 v1 版本装配成独立模块。
// 后续新增 v2/v3 时，只需要追加新的 factory，不需要回头改根路由实现。
func newAPIV1Module(deps WebServerDeps) apiModule {
	return apiv1.New(apiv1.Deps{
		DataStore:         deps.DataStore,
		DeviceRegistry:    deps.DeviceRegistry,
		DevicePresence:    deps.DevicePresence,
		DownlinkCommands:  deps.DownlinkCommands,
		Auth:              deps.Auth,
		Captcha:           deps.Captcha,
		Logger:            deps.Logger,
		Config:            deps.Config,
		LoginAttemptStore: apiv1.NewInMemoryLoginAttemptStore(),
	})
}

func (ws *webServer) registerAPIRoutes(mux *http.ServeMux) {
	for _, module := range ws.apiModules {
		module.RegisterRoutes(mux)
	}
}
