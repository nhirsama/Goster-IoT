package web

import (
	"net/http"

	apiv1 "github.com/nhirsama/Goster-IoT/src/web/v1"
)

func (ws *webServer) resolveAllowedAPIOrigin(r *http.Request, origin string) (string, bool) {
	return ws.apiV1Handler().ResolveAllowedOrigin(r, origin)
}

func isSameOriginRequest(r *http.Request, origin string) bool {
	return apiv1.IsSameOriginRequest(r, origin)
}
