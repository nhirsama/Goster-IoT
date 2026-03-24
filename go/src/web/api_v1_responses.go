package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
)

func (ws *webServer) apiOK(w http.ResponseWriter, r *http.Request, data interface{}) {
	ws.apiWrite(w, http.StatusOK, apiEnvelope{
		Code:      0,
		Message:   "ok",
		RequestID: ws.getRequestID(r),
		Data:      data,
	})
}

func (ws *webServer) apiError(w http.ResponseWriter, r *http.Request, httpStatus, code int, message string, detail *apiErrorDetail) {
	ws.apiErrorWithRequestID(w, httpStatus, ws.getRequestID(r), code, message, detail)
}

func (ws *webServer) apiErrorWithRequestID(w http.ResponseWriter, httpStatus int, requestID string, code int, message string, detail *apiErrorDetail) {
	ws.apiWrite(w, httpStatus, apiEnvelope{
		Code:      code,
		Message:   message,
		RequestID: requestID,
		Error:     detail,
	})
}

func (ws *webServer) apiMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	ws.apiError(w, r, http.StatusMethodNotAllowed, 40501, "method not allowed",
		&apiErrorDetail{Type: "method_not_allowed"})
}

func (ws *webServer) apiNoContent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Request-Id", ws.getRequestID(r))
	w.WriteHeader(http.StatusNoContent)
}

func (ws *webServer) apiWrite(w http.ResponseWriter, status int, payload apiEnvelope) {
	if payload.RequestID != "" {
		w.Header().Set("X-Request-Id", payload.RequestID)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (ws *webServer) getRequestID(r *http.Request) string {
	if rid, ok := r.Context().Value(apiCtxRequestID).(string); ok && rid != "" {
		return rid
	}
	if rid := r.Header.Get("X-Request-Id"); rid != "" {
		return rid
	}

	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err == nil {
		return "req_" + strconv.FormatInt(time.Now().Unix(), 10) + "_" + hex.EncodeToString(buf)
	}
	return "req_" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func (ws *webServer) maxAPIBodyBytes() int64 {
	return ws.webConfig().MaxAPIBodyBytes
}

func (ws *webServer) deviceListDefaultPageSize() int {
	return ws.webConfig().DeviceListPage.DefaultSize
}

func (ws *webServer) deviceListMaxPageSize() int {
	return ws.webConfig().DeviceListPage.MaxSize
}

func (ws *webServer) metricsMinValidTimestampMs() int64 {
	return ws.webConfig().Metrics.MinValidTimestampMs
}

func (ws *webServer) metricsDefaultRangeLabel() string {
	return ws.webConfig().Metrics.DefaultRangeLabel
}

func (ws *webServer) webConfig() appcfg.WebConfig {
	return appcfg.NormalizeWebConfig(ws.config)
}
