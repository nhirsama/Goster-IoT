package web

import (
	"net/http"
	"path/filepath"

	"github.com/aarondl/authboss/v3"
)

// registerRoutes 注册所有的 HTTP 路由
func (ws *webServer) registerRoutes(mux *http.ServeMux) {
	// 定义内部 Auth 处理器（处理剥离后的相对路径）
	authLogic := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 此时路径已经是 /register, /login 等
		if r.URL.Path == "/register" && r.Method == http.MethodPost {
			if !ws.turnstile.Verify(r) {
				ws.authboss.Config.Core.Redirector.Redirect(w, r, authboss.RedirectOptions{
					Code:         http.StatusFound,
					RedirectPath: "/auth/register",
					Failure:      "验证码验证失败，请重试",
				})
				return
			}
		}
		ws.authboss.Config.Core.Router.ServeHTTP(w, r)
	})

	// 挂载逻辑：剥离前缀 -> 加载 Session 环境 -> 执行业务逻辑
	mux.Handle("/auth/", http.StripPrefix("/auth", ws.authboss.LoadClientStateMiddleware(authLogic)))

	// 重定向旧的 Auth 路由
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
	})
	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/auth/logout", http.StatusFound)
	})

	staticPath := filepath.Join(ws.htmlDir, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticPath))))

	// 新版前后端分离 API 路由
	ws.registerAPIRoutes(mux)
}
