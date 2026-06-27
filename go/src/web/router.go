package web

import (
	"net/http"
)

// registerRoutes 注册所有的 HTTP 路由
func (ws *webServer) registerRoutes(mux *http.ServeMux) {
	// 健康检查端点（无需认证）
	mux.HandleFunc("/health", ws.healthCheckHandler)
	mux.HandleFunc("/readiness", ws.readinessCheckHandler)

	// 仅暴露契约化 API 路由，SSR 模板路由已移除。
	ws.registerAPIRoutes(mux)
}
