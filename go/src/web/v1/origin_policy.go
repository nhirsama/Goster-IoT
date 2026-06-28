package v1

import (
	"net/http"
	"net/url"
	"strings"
)

// ResolveAllowedOrigin 按同源或白名单策略校验调用方 Origin。
// 当使用凭证（credentials）时，禁止使用通配符 "*"。
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
		// 禁止在携带凭证的情况下使用通配符 "*"
		// 这会导致浏览器拒绝响应（CORS 安全策略）
		if candidate == "*" {
			// 通配符模式：不支持凭证，直接拒绝
			continue
		}
		if candidate == origin {
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
