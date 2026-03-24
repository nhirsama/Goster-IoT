package v1

import (
	"net/http"
	"net/url"
	"strings"
)

// ResolveAllowedOrigin 按同源或白名单策略校验调用方 Origin。
func (api *API) ResolveAllowedOrigin(r *http.Request, origin string) (string, bool) {
	if IsSameOriginRequest(r, origin) {
		return origin, true
	}

	raw := api.webConfig().APICORSAllowOrigins
	for _, candidate := range strings.Split(raw, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if candidate == "*" || candidate == origin {
			return origin, true
		}
	}
	return "", false
}

// IsSameOriginRequest 使用真实请求 Host 与浏览器 Origin 做同源判断。
func IsSameOriginRequest(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if !strings.EqualFold(u.Host, r.Host) {
		return false
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return strings.EqualFold(u.Scheme, scheme)
}
