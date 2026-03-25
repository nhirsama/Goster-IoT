package v1

import (
	"context"
	"net/http"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
)

// Middleware 为所有 v1 路由注入请求 ID，并统一处理 CORS。
func (api *API) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := api.requestID(r)
		ctx := context.WithValue(r.Context(), ContextRequestID, rid)
		ctx = logger.IntoContext(ctx, api.log().With(
			inter.String("request_id", rid),
			inter.String("method", r.Method),
			inter.String("path", r.URL.Path),
		))
		r = r.WithContext(ctx)

		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			allowOrigin, ok := api.ResolveAllowedOrigin(r, origin)
			if !ok {
				api.Error(w, r, http.StatusForbidden, 40302, "origin not allowed",
					&ErrorDetail{Type: "forbidden_origin", Field: "origin"})
				return
			}
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", allowOrigin)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id, X-Tenant-Id")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Expose-Headers", "X-Request-Id")
		}

		if r.Method == http.MethodOptions {
			api.NoContent(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware 完成认证、权限检查和当前租户范围解析。
func (api *API) AuthMiddleware(next http.HandlerFunc, minPerm inter.PermissionType) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := api.requestID(r)

		u, err := api.auth.CurrentUser(r)
		if err != nil || u == nil {
			api.ErrorWithRequestID(w, http.StatusUnauthorized, rid, 40101, "unauthorized",
				&ErrorDetail{Type: "auth_required"})
			return
		}

		user, ok := u.(inter.SessionUser)
		if !ok {
			api.ErrorWithRequestID(w, http.StatusInternalServerError, rid, 50001, "invalid user type",
				&ErrorDetail{Type: "internal_error"})
			return
		}

		if user.GetPermission() < minPerm {
			api.ErrorWithRequestID(w, http.StatusForbidden, rid, 40301, "forbidden",
				&ErrorDetail{Type: "permission_denied"})
			return
		}

		requestedTenant := api.requestedTenantID(r)
		principal, err := api.principalResolver.Resolve(r.Context(), user, requestedTenant)
		if err != nil {
			if isTenantAccessError(err) {
				api.ErrorWithRequestID(w, http.StatusForbidden, rid, 40303, "forbidden",
					&ErrorDetail{Type: "tenant_access_denied", Field: "tenant_id"})
				return
			}
			api.InternalError(w, r, 50013, err)
			return
		}

		ctx := context.WithValue(r.Context(), ContextRequestID, rid)
		ctx = context.WithValue(ctx, ContextUsername, principal.Username)
		ctx = context.WithValue(ctx, ContextPerm, principal.Permission)
		ctx = context.WithValue(ctx, ContextTenantID, principal.Scope.TenantID)
		ctx = logger.IntoContext(ctx, logger.FromContext(ctx).With(
			inter.String("username", principal.Username),
			inter.Int("permission", int(principal.Permission)),
			inter.String("requested_tenant_id", requestedTenant),
			inter.String("tenant_id", principal.Scope.TenantID),
		))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
