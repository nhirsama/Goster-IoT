package web

import (
	"fmt"
	"net/http"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// userListHandler 用户列表处理 (仅管理员)
func (ws *webServer) userListHandler(w http.ResponseWriter, r *http.Request) {
	users, err := ws.dataStore.ListUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ws.templates["user_list.html"].Execute(w, users)
}

// updateUserPermissionHandler 更新用户权限处理 (仅管理员)
func (ws *webServer) updateUserPermissionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
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
