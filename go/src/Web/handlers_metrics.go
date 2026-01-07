package Web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// MetricsPageData 监控页面数据结构
type MetricsPageData struct {
	UUID       string
	Meta       inter.DeviceMetadata
	Metrics    []inter.MetricPoint
	Range      string
	Permission int
}

// metricsHandler 监控页面处理
func (ws *webServer) metricsHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Path[len("/metrics/"):]

	u, _ := ws.authboss.CurrentUser(r)
	var perm inter.PermissionType
	if user, ok := u.(inter.SessionUser); ok {
		perm = user.GetPermission()
	}

	meta, err := ws.dataStore.LoadConfig(uuid)
	if err != nil {
		http.Error(w, "未找到设备: "+err.Error(), http.StatusNotFound)
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
		Permission: int(perm),
	}

	ws.templates["metrics.html"].Execute(w, data)
}

// apiMetricsHandler JSON 格式监控数据接口
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
