package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

type webServer struct {
	dataStore     inter.DataStore
	deviceManager inter.DeviceManager
	templates     map[string]*template.Template
}

func NewWebServer(ds inter.DataStore, dm inter.DeviceManager) inter.WebServer {
	return &webServer{
		dataStore:     ds,
		deviceManager: dm,
		templates:     loadTemplates(),
	}
}

func (ws *webServer) Start() {
	http.HandleFunc("/", ws.indexHandler)
	http.HandleFunc("/devices", ws.deviceListHandler)
	http.HandleFunc("/devices/pending", ws.pendingListHandler)
	http.HandleFunc("/devices/blacklist", ws.blacklistHandler)
	http.HandleFunc("/device/approve", ws.approveHandler)
	http.HandleFunc("/device/revoke", ws.revokeHandler)
	http.HandleFunc("/device/delete", ws.deleteHandler)
	http.HandleFunc("/device/unblock", ws.unblockHandler)
	http.HandleFunc("/device/token/refresh", ws.refreshTokenHandler)
	http.HandleFunc("/metrics/", ws.metricsHandler)
	http.HandleFunc("/api/metrics/", ws.apiMetricsHandler)

	fmt.Println("Starting web server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func loadTemplates() map[string]*template.Template {
	templates := make(map[string]*template.Template)
	templates["index.html"] = template.Must(template.ParseFiles("go/html/index.html"))
	templates["device_list.html"] = template.Must(template.ParseFiles("go/html/device_list.html"))
	templates["metrics.html"] = template.Must(template.ParseFiles("go/html/metrics.html"))
	templates["pending_list.html"] = template.Must(template.ParseFiles("go/html/pending_list.html"))
	templates["blacklist.html"] = template.Must(template.ParseFiles("go/html/blacklist.html"))
	return templates
}

func (ws *webServer) indexHandler(w http.ResponseWriter, r *http.Request) {
	ws.templates["index.html"].Execute(w, nil)
}

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
		// Filter out pending and refused devices from the main list
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

func (ws *webServer) unblockHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	meta, err := ws.dataStore.LoadConfig(uuid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	meta.AuthenticateStatus = inter.AuthenticatePending
	ws.dataStore.SaveMetadata(uuid, meta)

	ws.blacklistHandler(w, r)
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

	// Trigger refresh of pending list
	ws.pendingListHandler(w, r)
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

	ws.pendingListHandler(w, r)
}

func (ws *webServer) deleteHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	ws.dataStore.DestroyDevice(uuid)

	// Return a script to redirect or clear the view
	w.Header().Set("HX-Location", "/")
}

func (ws *webServer) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	newToken := fmt.Sprintf("tk-%d-%s", time.Now().Unix(), uuid[:8]) // Simple token generation
	ws.dataStore.UpdateToken(uuid, newToken)

	// Redirect back to metrics view to see new token
	w.Header().Set("HX-Trigger", "refreshMetrics")
	http.Redirect(w, r, "/metrics/"+uuid, http.StatusSeeOther)
}

type MetricsPageData struct {
	UUID    string
	Meta    inter.DeviceMetadata
	Metrics []inter.MetricPoint
	Range   string
}

func (ws *webServer) metricsHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Path[len("/metrics/"):]
	// Handle query params being part of the path if not careful, but http.StripPrefix usually handles this?
	// Actually r.URL.Path usually doesn't include query params.
	// But wait, the previous handler was `uuid := r.URL.Path[len("/metrics/"):]`.
	// If I request `/metrics/uuid?range=1h`, Path is `/metrics/uuid`. Correct.

	meta, err := ws.dataStore.LoadConfig(uuid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		UUID:    uuid,
		Meta:    meta,
		Metrics: metrics,
		Range:   rangeParam,
	}

	ws.templates["metrics.html"].Execute(w, data)
}

func (ws *webServer) apiMetricsHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Path[len("/api/metrics/"):]

	// Query last 1 hour of data
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
