package web

import (
	"log"
	"net/http"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

func (ws *webServer) apiCanViewDeviceToken(r *http.Request) bool {
	perm, _ := r.Context().Value(apiCtxPerm).(inter.PermissionType)
	return perm >= inter.PermissionReadWrite
}

func apiDeviceMetaData(meta inter.DeviceMetadata, includeToken bool) map[string]interface{} {
	token := interface{}(nil)
	if includeToken {
		token = meta.Token
	}
	return map[string]interface{}{
		"name":                meta.Name,
		"hw_version":          meta.HWVersion,
		"sw_version":          meta.SWVersion,
		"config_version":      meta.ConfigVersion,
		"sn":                  meta.SerialNumber,
		"mac":                 meta.MACAddress,
		"created_at":          meta.CreatedAt,
		"token":               token,
		"authenticate_status": int(meta.AuthenticateStatus),
	}
}

func countAdminUsers(users []inter.User) int {
	count := 0
	for _, u := range users {
		if u.Permission == inter.PermissionAdmin {
			count++
		}
	}
	return count
}

func (ws *webServer) apiInternalError(w http.ResponseWriter, r *http.Request, code int, err error) {
	requestID := ws.getRequestID(r)
	if err != nil {
		log.Printf("api internal error code=%d request_id=%s method=%s path=%s err=%v",
			code, requestID, r.Method, r.URL.Path, err)
	}
	ws.apiErrorWithRequestID(w, http.StatusInternalServerError, requestID, code, "internal server error",
		&apiErrorDetail{Type: "internal_error"})
}
