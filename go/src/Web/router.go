package Web

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/dchest/captcha"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

// registerRoutes 注册所有的 HTTP 路由
func (ws *webServer) registerRoutes(mux *http.ServeMux) {
	// Mount Authboss
	// Note: Authboss router handles /auth/* routes
	mux.Handle("/auth/", http.StripPrefix("/auth", ws.authboss.Config.Core.Router))

	// Redirect old routes to new Authboss routes or handle them
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
	})
	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/auth/logout", http.StatusFound)
	})

	// Keep Captcha for other uses if needed
	mux.Handle("/captcha/", captcha.Server(captcha.StdWidth, captcha.StdHeight))
	mux.HandleFunc("/api/captcha/new", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": captcha.New()})
	})

	staticPath := filepath.Join(ws.htmlDir, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticPath))))

	// Setup stack for protected routes: LoadClientState -> AuthMiddleware
	// We wrap the mux handler or individual handlers?
	// The best way is to wrap individual handlers because they have different permissions.
	// BUT, we need LoadClientStateMiddleware to be global or at least before AuthMiddleware.

	// Since we are using standard Mux, we have to wrap each handler manually or wrap the whole mux.
	// But wrapping the whole mux might interfere with public routes if not careful.
	// Authboss LoadClientStateMiddleware is safe to run on all routes (it just checks cookies).

	// Helper to chain middlewares
	chain := func(handler http.HandlerFunc, minPerm inter.PermissionType) http.Handler {
		// 1. AuthMiddleware (checks permission)
		h := ws.authMiddleware(handler, minPerm)
		// 2. LoadClientState (loads user from session)
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

// handleProtectedWithMux 注册受保护的路由到 mux (Removed in favor of direct mux.Handle with chain)

// authMiddleware 鉴权中间件
func (ws *webServer) authMiddleware(next http.Handler, minPerm inter.PermissionType) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if user is known
		u, err := ws.authboss.CurrentUser(r)
		if err != nil {
			// On error (e.g. database down), what to do?
			// For now treat as not logged in or error.
			ws.redirectOrHTMX(w, r, "/auth/login")
			return
		}
		if u == nil {
			ws.redirectOrHTMX(w, r, "/auth/login")
			return
		}

		// Cast to our User type
		user, ok := u.(inter.SessionUser)
		if !ok {
			http.Error(w, "Internal Server Error: Invalid User Type", http.StatusInternalServerError)
			return
		}

		// Check Permission
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
