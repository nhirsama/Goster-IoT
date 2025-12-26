package Web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/sessions"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

var store = sessions.NewCookieStore([]byte("super-secret-key-change-me"))

type webServer struct {
	dataStore     inter.DataStore
	deviceManager inter.DeviceManager
	templates     map[string]*template.Template
	htmlDir       string
}

func NewWebServer(ds inter.DataStore, dm inter.DeviceManager, htmlDir string) inter.WebServer {
	return &webServer{
		dataStore:     ds,
		deviceManager: dm,
		templates:     loadTemplates(htmlDir),
		htmlDir:       htmlDir,
	}
}

func (ws *webServer) Start() {
	// Public Routes
	http.HandleFunc("/login", ws.loginHandler)
	http.HandleFunc("/register", ws.registerHandler)
	http.HandleFunc("/logout", ws.logoutHandler)

	staticPath := filepath.Join(ws.htmlDir, "static")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticPath))))

	// Protected Routes
	// Root is accessible to all authenticated users (permission 0+), content varies by permission
	ws.handleProtected("/", ws.indexHandler, inter.PermissionNone)
	ws.handleProtected("/devices", ws.deviceListHandler, inter.PermissionReadOnly)
	ws.handleProtected("/devices/pending", ws.pendingListHandler, inter.PermissionReadWrite) // Approving requires RW (2)
	ws.handleProtected("/devices/blacklist", ws.blacklistHandler, inter.PermissionReadOnly)

	// Actions require ReadWrite (2)
	ws.handleProtected("/device/approve", ws.approveHandler, inter.PermissionReadWrite)
	ws.handleProtected("/device/revoke", ws.revokeHandler, inter.PermissionReadWrite)
	ws.handleProtected("/device/delete", ws.deleteHandler, inter.PermissionReadWrite)
	ws.handleProtected("/device/unblock", ws.unblockHandler, inter.PermissionReadWrite)
	ws.handleProtected("/device/token/refresh", ws.refreshTokenHandler, inter.PermissionReadWrite)

	// Metrics viewing requires ReadOnly (1)
	ws.handleProtected("/metrics/", ws.metricsHandler, inter.PermissionReadOnly)
	ws.handleProtected("/api/metrics/", ws.apiMetricsHandler, inter.PermissionReadOnly)

	// User Management Routes (Admin only)
	ws.handleProtected("/users", ws.userListHandler, inter.PermissionAdmin)
	ws.handleProtected("/user/permission", ws.updateUserPermissionHandler, inter.PermissionAdmin)

	// New Page Views for Main Content
	ws.handleProtected("/devices/view/pending", ws.pendingPageHandler, inter.PermissionReadWrite)
	ws.handleProtected("/devices/view/blacklist", ws.blacklistPageHandler, inter.PermissionReadOnly)

	fmt.Println("Starting Web server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (ws *webServer) handleProtected(pattern string, handler http.HandlerFunc, minPerm inter.PermissionType) {
	http.Handle(pattern, ws.authMiddleware(handler, minPerm))
}

func (ws *webServer) authMiddleware(next http.Handler, minPerm inter.PermissionType) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "session-name")
		auth, ok := session.Values["authenticated"].(bool)

		if !ok || !auth {
			ws.redirectOrHTMX(w, r, "/login")
			return
		}

		// Fetch fresh permission from DB
		username, ok := session.Values["username"].(string)
		if !ok {
			ws.redirectOrHTMX(w, r, "/login")
			return
		}

		userPerm, err := ws.dataStore.GetUserPermission(username)
		if err != nil {
			// User might be deleted
			ws.redirectOrHTMX(w, r, "/login")
			return
		}

		if userPerm < minPerm {
			http.Error(w, "Forbidden: Insufficient Permissions (权限不足)", http.StatusForbidden)
			return
		}

		// Store permission in context if needed, or rely on handlers fetching it again
		// (or optimize by putting it in context). For now, handlers that need it call DB or we assume middleware passed.
		// But indexHandler uses session values. We should update session or indexHandler.
		// Updating session here ensures next request has it, but current request logic in handlers might read stale session.
		// Better to update session value here so downstream handlers can use it (although they should ideally read from DB or Context).
		// We will simply update the session value to keep it somewhat in sync, but rely on DB for the check.
		session.Values["permission"] = int(userPerm)
		session.Save(r, w)

		next.ServeHTTP(w, r)
	})
}

func (ws *webServer) redirectOrHTMX(w http.ResponseWriter, r *http.Request, url string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, url, http.StatusFound)
	}
}

func (ws *webServer) loginHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")

	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		perm, err := ws.dataStore.LoginUser(username, password)
		if err != nil {
			ws.templates["login.html"].Execute(w, map[string]interface{}{"Error": "用户名或密码错误"})
			return
		}

		session.Values["authenticated"] = true
		session.Values["username"] = username
		session.Values["permission"] = int(perm)
		session.Save(r, w)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	ws.templates["login.html"].Execute(w, nil)
}

func (ws *webServer) registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		count, err := ws.dataStore.GetUserCount()
		if err != nil {
			http.Error(w, "Database Error", http.StatusInternalServerError)
			return
		}

		var perm inter.PermissionType
		if count == 0 {
			perm = inter.PermissionAdmin
		} else {
			perm = inter.PermissionNone // Default to None
		}

		err = ws.dataStore.RegisterUser(username, password, perm)
		if err != nil {
			http.Error(w, "Registration failed: "+err.Error(), http.StatusBadRequest)
			return
		}

		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if t, ok := ws.templates["register.html"]; ok {
		t.Execute(w, nil)
	} else {
		http.Error(w, "Register template missing", http.StatusInternalServerError)
	}
}

func (ws *webServer) logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	session.Values["authenticated"] = false
	session.Values["username"] = ""
	session.Values["permission"] = 0
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func loadTemplates(htmlDir string) map[string]*template.Template {
	templates := make(map[string]*template.Template)

	funcMap := template.FuncMap{
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"hasPerm": func(userPerm int, reqPerm int) bool {
			return userPerm >= reqPerm
		},
	}

	parse := func(name, file string) *template.Template {
		t := template.New(name).Funcs(funcMap)
		t = template.Must(t.ParseFiles(filepath.Join(htmlDir, file)))
		return t
	}

	templates["index.html"] = parse("index.html", "index.html")
	templates["login.html"] = parse("login.html", "login.html")
	templates["register.html"] = parse("register.html", "register.html")
	templates["device_list.html"] = parse("device_list.html", "device_list.html")
	templates["metrics.html"] = parse("metrics.html", "metrics.html")
	templates["pending_list.html"] = parse("pending_list.html", "pending_list.html")
	templates["blacklist.html"] = parse("blacklist.html", "blacklist.html")
	templates["user_list.html"] = parse("user_list.html", "user_list.html")
	templates["pending_table.html"] = parse("pending_table.html", "pending_table.html")
	templates["blacklist_table.html"] = parse("blacklist_table.html", "blacklist_table.html")
	return templates
}

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

// User Management Handlers

func (ws *webServer) userListHandler(w http.ResponseWriter, r *http.Request) {
	users, err := ws.dataStore.ListUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ws.templates["user_list.html"].Execute(w, users)
}

func (ws *webServer) updateUserPermissionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.FormValue("username")
	permStr := r.FormValue("permission")

	var perm inter.PermissionType
	var permInt int
	fmt.Sscanf(permStr, "%d", &permInt)
	perm = inter.PermissionType(permInt)

	err := ws.dataStore.UpdateUserPermission(username, perm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ws.handleActionResponse(w, r, ws.userListHandler, "user-list")
}

// Device Handlers

type DeviceListView struct {
	inter.DeviceRecord
	Status       string
	StatusString string
}

func (ws *webServer) deviceListHandler(w http.ResponseWriter, r *http.Request) {
	devices, err := ws.dataStore.ListDevices(1, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var viewData []DeviceListView
	for _, d := range devices {
		if d.Meta.AuthenticateStatus == inter.AuthenticatePending || d.Meta.AuthenticateStatus == inter.AuthenticateRefuse {
			continue
		}

		status, _ := ws.deviceManager.QueryDeviceStatus(d.UUID)
		statusStr := "离线"
		statusClass := "text-secondary"

		switch status {
		case inter.StatusOnline:
			statusStr = "在线"
			statusClass = "text-success"
		case inter.StatusDelayed:
			statusStr = "延迟"
			statusClass = "text-warning"
		case inter.StatusOffline:
			statusStr = "离线"
			statusClass = "text-danger"
		}

		viewData = append(viewData, DeviceListView{
			DeviceRecord: d,
			Status:       statusClass,
			StatusString: statusStr,
		})
	}

	ws.templates["device_list.html"].Execute(w, viewData)
}

func (ws *webServer) pendingListHandler(w http.ResponseWriter, r *http.Request) {
	devices, err := ws.dataStore.ListDevices(1, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var pendingDevices []inter.DeviceRecord
	for _, d := range devices {
		if d.Meta.AuthenticateStatus == inter.AuthenticatePending {
			pendingDevices = append(pendingDevices, d)
		}
	}

	ws.templates["pending_list.html"].Execute(w, pendingDevices)
}

func (ws *webServer) blacklistHandler(w http.ResponseWriter, r *http.Request) {
	devices, err := ws.dataStore.ListDevices(1, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var blacklistedDevices []inter.DeviceRecord
	for _, d := range devices {
		if d.Meta.AuthenticateStatus == inter.AuthenticateRefuse {
			blacklistedDevices = append(blacklistedDevices, d)
		}
	}

	ws.templates["blacklist.html"].Execute(w, blacklistedDevices)
}

func (ws *webServer) handleActionResponse(w http.ResponseWriter, r *http.Request, listHandler http.HandlerFunc, targetID string) {
	if r.Header.Get("HX-Target") == targetID {
		listHandler(w, r)
	} else if r.Header.Get("HX-Target") == "main-view" {
		if targetID == "user-list" {
			ws.userListHandler(w, r)
			return
		}
		w.Header().Set("HX-Location", "/")
		w.WriteHeader(http.StatusOK)
	} else {
		w.Header().Set("HX-Location", "/")
		w.WriteHeader(http.StatusOK)
	}
}

func (ws *webServer) pendingPageHandler(w http.ResponseWriter, r *http.Request) {
	devices, err := ws.dataStore.ListDevices(1, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var pendingDevices []inter.DeviceRecord
	for _, d := range devices {
		if d.Meta.AuthenticateStatus == inter.AuthenticatePending {
			pendingDevices = append(pendingDevices, d)
		}
	}
	ws.templates["pending_table.html"].Execute(w, pendingDevices)
}

func (ws *webServer) blacklistPageHandler(w http.ResponseWriter, r *http.Request) {
	devices, err := ws.dataStore.ListDevices(1, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var blacklistedDevices []inter.DeviceRecord
	for _, d := range devices {
		if d.Meta.AuthenticateStatus == inter.AuthenticateRefuse {
			blacklistedDevices = append(blacklistedDevices, d)
		}
	}
	ws.templates["blacklist_table.html"].Execute(w, blacklistedDevices)
}

func (ws *webServer) unblockHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	meta, err := ws.dataStore.LoadConfig(uuid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	meta.AuthenticateStatus = inter.AuthenticatePending
	ws.dataStore.SaveMetadata(uuid, meta)

	if r.Header.Get("HX-Target") == "main-view" {
		ws.blacklistPageHandler(w, r)
	} else {
		ws.handleActionResponse(w, r, ws.blacklistHandler, "blacklist-view")
	}
}

func (ws *webServer) approveHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	meta, err := ws.dataStore.LoadConfig(uuid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	meta.AuthenticateStatus = inter.Authenticated
	ws.dataStore.SaveMetadata(uuid, meta)

	if r.Header.Get("HX-Target") == "main-view" {
		ws.pendingPageHandler(w, r)
	} else {
		ws.handleActionResponse(w, r, ws.pendingListHandler, "pending-list")
	}
}

func (ws *webServer) revokeHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	meta, err := ws.dataStore.LoadConfig(uuid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	meta.AuthenticateStatus = inter.AuthenticateRefuse
	ws.dataStore.SaveMetadata(uuid, meta)

	if r.Header.Get("HX-Target") == "main-view" {
		ws.pendingPageHandler(w, r)
	} else {
		ws.handleActionResponse(w, r, ws.pendingListHandler, "pending-list")
	}
}

func (ws *webServer) deleteHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	ws.dataStore.DestroyDevice(uuid)

	w.Header().Set("HX-Location", "/")
}

func (ws *webServer) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	newToken := fmt.Sprintf("tk-%d-%s", time.Now().Unix(), uuid[:8])
	ws.dataStore.UpdateToken(uuid, newToken)

	w.Header().Set("HX-Trigger", "refreshMetrics")
	http.Redirect(w, r, "/metrics/"+uuid, http.StatusSeeOther)
}

type MetricsPageData struct {
	UUID       string
	Meta       inter.DeviceMetadata
	Metrics    []inter.MetricPoint
	Range      string
	Permission int // Added Permission field
}

func (ws *webServer) metricsHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Path[len("/metrics/"):]

	session, _ := store.Get(r, "session-name")
	username, _ := session.Values["username"].(string)

	perm, err := ws.dataStore.GetUserPermission(username)
	if err != nil {
		perm = inter.PermissionNone
	}

	meta, err := ws.dataStore.LoadConfig(uuid)
	if err != nil {
		http.Error(w, "Device not found: "+err.Error(), http.StatusNotFound)
		return
	}

	rangeParam := r.URL.Query().Get("range")
	if rangeParam == "" {
		rangeParam = "1h"
	}

	var duration time.Duration
	var start int64
	end := time.Now().Unix()

	if rangeParam == "all" {
		start = 0
	} else {
		switch rangeParam {
		case "1h":
			duration = time.Hour
		case "6h":
			duration = 6 * time.Hour
		case "24h":
			duration = 24 * time.Hour
		case "7d":
			duration = 7 * 24 * time.Hour
		default:
			rangeParam = "1h"
			duration = time.Hour
		}
		start = time.Now().Add(-duration).Unix()
	}

	metrics, err := ws.dataStore.QueryMetrics(uuid, start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := MetricsPageData{
		UUID:       uuid,
		Meta:       meta,
		Metrics:    metrics,
		Range:      rangeParam,
		Permission: int(perm), // Pass fresh permission
	}

	ws.templates["metrics.html"].Execute(w, data)
}

func (ws *webServer) apiMetricsHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Path[len("/api/metrics/"):]

	end := time.Now().Unix()
	start := end - 3600

	metrics, err := ws.dataStore.QueryMetrics(uuid, start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}
