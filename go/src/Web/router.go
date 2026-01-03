package Web

import (
	"net/http"
	"path/filepath"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// registerRoutes 注册所有的 HTTP 路由
func (ws *webServer) registerRoutes(mux *http.ServeMux) {
	// 挂载 Authboss 路由
	// 注意: 必须使用 LoadClientStateMiddleware 包装 Router 以处理 Session/Cookie
	authHandler := ws.authboss.LoadClientStateMiddleware(ws.authboss.Config.Core.Router)
	mux.Handle("/auth/", http.StripPrefix("/auth", authHandler))

	// 重定向旧的 Auth 路由到新路径
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
	})
	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/auth/logout", http.StatusFound)
	})

	staticPath := filepath.Join(ws.htmlDir, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticPath))))

	// 辅助函数: 链式调用中间件 (AuthMiddleware -> LoadClientStateMiddleware)
	chain := func(handler http.HandlerFunc, minPerm inter.PermissionType) http.Handler {
		// 1. AuthMiddleware (检查权限)
		h := ws.authMiddleware(handler, minPerm)
		// 2. LoadClientState (从 Session 加载用户)
		return ws.authboss.LoadClientStateMiddleware(h)
	}

	// 受保护的路由
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
		if err != nil {
			// 如果出错 (如数据库连接失败)，视为未登录
			ws.redirectOrHTMX(w, r, "/auth/login")
			return
		}
		if u == nil {
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
