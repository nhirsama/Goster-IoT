package Web

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/dchest/captcha"
	"github.com/gorilla/sessions"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

var store = sessions.NewCookieStore([]byte("super-secret-key-change-me"))

// registerRoutes 注册所有的 HTTP 路由
func (ws *webServer) registerRoutes(mux *http.ServeMux) {
	// 公开路由
	mux.HandleFunc("/login", ws.loginHandler)
	mux.HandleFunc("/register", ws.registerHandler)
	mux.HandleFunc("/logout", ws.logoutHandler)
	mux.Handle("/captcha/", captcha.Server(captcha.StdWidth, captcha.StdHeight))
	mux.HandleFunc("/api/captcha/new", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": captcha.New()})
	})

	staticPath := filepath.Join(ws.htmlDir, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticPath))))

	// 受保护的路由
	ws.handleProtectedWithMux(mux, "/", ws.indexHandler, inter.PermissionNone)
	ws.handleProtectedWithMux(mux, "/devices", ws.deviceListHandler, inter.PermissionReadOnly)
	ws.handleProtectedWithMux(mux, "/devices/pending", ws.pendingListHandler, inter.PermissionReadWrite)
	ws.handleProtectedWithMux(mux, "/devices/blacklist", ws.blacklistHandler, inter.PermissionReadOnly)

	ws.handleProtectedWithMux(mux, "/device/approve", ws.approveHandler, inter.PermissionReadWrite)
	ws.handleProtectedWithMux(mux, "/device/revoke", ws.revokeHandler, inter.PermissionReadWrite)
	ws.handleProtectedWithMux(mux, "/device/delete", ws.deleteHandler, inter.PermissionReadWrite)
	ws.handleProtectedWithMux(mux, "/device/unblock", ws.unblockHandler, inter.PermissionReadWrite)
	ws.handleProtectedWithMux(mux, "/device/token/refresh", ws.refreshTokenHandler, inter.PermissionReadWrite)

	ws.handleProtectedWithMux(mux, "/metrics/", ws.metricsHandler, inter.PermissionReadOnly)
	ws.handleProtectedWithMux(mux, "/api/metrics/", ws.apiMetricsHandler, inter.PermissionReadOnly)

	ws.handleProtectedWithMux(mux, "/users", ws.userListHandler, inter.PermissionAdmin)
	ws.handleProtectedWithMux(mux, "/user/permission", ws.updateUserPermissionHandler, inter.PermissionAdmin)

	ws.handleProtectedWithMux(mux, "/devices/view/pending", ws.pendingPageHandler, inter.PermissionReadWrite)
	ws.handleProtectedWithMux(mux, "/devices/view/blacklist", ws.blacklistPageHandler, inter.PermissionReadOnly)
}

// handleProtectedWithMux 注册受保护的路由到 mux
func (ws *webServer) handleProtectedWithMux(mux *http.ServeMux, pattern string, handler http.HandlerFunc, minPerm inter.PermissionType) {
	mux.Handle(pattern, ws.authMiddleware(handler, minPerm))
}

// authMiddleware 鉴权中间件
func (ws *webServer) authMiddleware(next http.Handler, minPerm inter.PermissionType) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "session-name")
		auth, ok := session.Values["authenticated"].(bool)

		if !ok || !auth {
			ws.redirectOrHTMX(w, r, "/login")
			return
		}

		// 从 DB 获取最新的权限信息
		username, ok := session.Values["username"].(string)
		if !ok {
			ws.redirectOrHTMX(w, r, "/login")
			return
		}

		userPerm, err := ws.dataStore.GetUserPermission(username)
		if err != nil {
			// 用户可能已被删除
			ws.redirectOrHTMX(w, r, "/login")
			return
		}

		if userPerm < minPerm {
			http.Error(w, "Forbidden: Insufficient Permissions (权限不足)", http.StatusForbidden)
			return
		}

		// 更新 session 中的权限缓存
		session.Values["permission"] = int(userPerm)
		session.Save(r, w)

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
	session, _ := store.Get(r, "session-name")
	username, _ := session.Values["username"].(string)

	perm, err := ws.dataStore.GetUserPermission(username)
	if err != nil {
		perm = inter.PermissionNone
	}

	ws.templates["index.html"].Execute(w, map[string]interface{}{
		"Permission": int(perm),
		"IsZeroPerm": perm == inter.PermissionNone,
	})
}
