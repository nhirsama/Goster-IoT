package v1

import (
	"net/http"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/device_manager"
)

const maxAccessControlMetricTimestampMs int64 = 1<<63 - 1

// AccessControlHandler 返回设备门禁模块的当前计算状态。
func (api *API) AccessControlHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.MethodNotAllowed(w, r)
		return
	}

	uuid := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/access-control/"), "/")
	if uuid == "" {
		api.Error(w, r, http.StatusNotFound, 40451, "device not found",
			&ErrorDetail{Type: "not_found", Field: "uuid"})
		return
	}
	if !api.ensureDeviceExists(w, r, uuid) {
		return
	}

	points, err := api.dataStore.QueryMetricsByTenant(api.tenantID(r), uuid, 0, maxAccessControlMetricTimestampMs)
	if err != nil {
		api.InternalError(w, r, 50051, err)
		return
	}

	state := device_manager.EvaluateAccessControl(points)
	api.OK(w, r, map[string]interface{}{
		"uuid":            uuid,
		"signal_a":        state.SignalA,
		"signal_b":        state.SignalB,
		"open":            state.Open,
		"evaluated_at_ms": state.EvaluatedAtMs,
		"status_text":     state.StatusText,
		"rule":            "signal_a == 1 && signal_b == 1",
	})
}
