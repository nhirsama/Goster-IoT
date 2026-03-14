package web

import (
	"net/http"
)

// registerRoutes 注册所有的 HTTP 路由
func (ws *webServer) registerRoutes(mux *http.ServeMux) {
	// 仅暴露契约化 API 路由，SSR 模板路由已移除。
	ws.registerAPIRoutes(mux)
}
