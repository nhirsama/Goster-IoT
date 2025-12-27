package Web

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// DeviceListView 用于设备列表页面的视图数据
type DeviceListView struct {
	inter.DeviceRecord
	Status       string
	StatusString string
}

// deviceListHandler 设备列表页面处理
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

// pendingListHandler 待审核设备页面处理
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

// blacklistHandler 黑名单页面处理
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

// handleActionResponse 处理 HTMX 动作响应辅助函数
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

// pendingPageHandler 待审核表格局部视图处理
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

// blacklistPageHandler 黑名单表格局部视图处理
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

// unblockHandler 解除屏蔽操作处理
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

// approveHandler 通过审核操作处理
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

// revokeHandler 拒绝/吊销操作处理
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

// deleteHandler 删除设备操作处理
func (ws *webServer) deleteHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	ws.dataStore.DestroyDevice(uuid)

	w.Header().Set("HX-Location", "/")
}

// refreshTokenHandler 刷新 Token 操作处理
func (ws *webServer) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	newToken := fmt.Sprintf("tk-%d-%s", time.Now().Unix(), uuid[:8])
	ws.dataStore.UpdateToken(uuid, newToken)

	w.Header().Set("HX-Trigger", "refreshMetrics")
	http.Redirect(w, r, "/metrics/"+uuid, http.StatusSeeOther)
}
