package Web

import (
	"net/http"
	"path/filepath"

	"github.com/aarondl/authboss/v3"
	"github.com/nhirsama/Goster-IoT/src/inter"
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

	// 辅助函数: 链式调用中间件
	chain := func(handler http.HandlerFunc, minPerm inter.PermissionType) http.Handler {
		h := ws.authMiddleware(handler, minPerm)
		return ws.authboss.LoadClientStateMiddleware(h)
	}

	// 受保护的业务路由
	mux.Handle("/", chain(ws.indexHandler, inter.PermissionNone))
	mux.Handle("/devices", chain(ws.deviceListHandler, inter.PermissionReadOnly))
	mux.Handle("/devices/pending", chain(ws.pendingListHandler, inter.PermissionReadWrite))
	mux.Handle("/devices/blacklist", chain(ws.blacklistHandler, inter.PermissionReadOnly))
	mux.Handle("/device/approve", chain(ws.approveHandler, inter.PermissionReadWrite))
	mux.Handle("/device/revoke", chain(ws.revokeHandler, inter.PermissionReadWrite))
	mux.Handle("/device/delete", chain(ws.deleteHandler, inter.PermissionReadWrite))
	mux.Handle("/device/unblock", chain(ws.unblockHandler, inter.PermissionReadWrite))
	mux.Handle("/device/token/refresh", chain(ws.refreshTokenHandler, inter.PermissionReadWrite))
	mux.Handle("/metrics/", chain(ws.metricsHandler, inter.PermissionReadOnly))
	mux.Handle("/api/metrics/", chain(ws.apiMetricsHandler, inter.PermissionReadOnly))
	mux.Handle("/users", chain(ws.userListHandler, inter.PermissionAdmin))
	mux.Handle("/user/permission", chain(ws.updateUserPermissionHandler, inter.PermissionAdmin))
	mux.Handle("/devices/view/pending", chain(ws.pendingPageHandler, inter.PermissionReadWrite))
	mux.Handle("/devices/view/blacklist", chain(ws.blacklistPageHandler, inter.PermissionReadOnly))
}

// authMiddleware 鉴权中间件
func (ws *webServer) authMiddleware(next http.Handler, minPerm inter.PermissionType) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 获取当前用户
		u, err := ws.authboss.CurrentUser(r)
		if err != nil || u == nil {
			ws.redirectOrHTMX(w, r, "/auth/login")
			return
		}

		// 类型断言为 inter.SessionUser 接口
		user, ok := u.(inter.SessionUser)
		if !ok {
			http.Error(w, "Internal Server Error: Invalid User Type", http.StatusInternalServerError)
			return
		}

		// 检查权限
		if user.GetPermission() < minPerm {
			http.Error(w, "Forbidden: Insufficient Permissions (权限不足)", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// redirectOrHTMX 根据请求类型进行重定向 (支持 HTMX)
func (ws *webServer) redirectOrHTMX(w http.ResponseWriter, r *http.Request, url string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, url, http.StatusFound)
	}
}

// indexHandler 首页处理
func (ws *webServer) indexHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := ws.authboss.CurrentUser(r)
	perm := inter.PermissionNone
	username := "Guest"

	if u != nil {
		if user, ok := u.(inter.SessionUser); ok {
			perm = user.GetPermission()
			username = user.GetUsername()
		}
	}

	ws.templates["index.html"].Execute(w, map[string]interface{}{
		"Permission": int(perm),
		"IsZeroPerm": perm == inter.PermissionNone,
		"Username":   username,
	})
}
