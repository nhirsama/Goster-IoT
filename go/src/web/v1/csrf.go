package v1

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

const (
	csrfTokenCookieName = "csrf_token"
	csrfTokenHeaderName = "X-CSRF-Token"
	csrfTokenLength     = 32
	csrfTokenTTL        = 24 * time.Hour
)

// CSRFTokenStore 管理 CSRF token 的生成和验证
type CSRFTokenStore interface {
	GenerateToken(sessionID string) (string, error)
	ValidateToken(sessionID, token string) bool
}

// InMemoryCSRFStore 是基于内存的 CSRF token 存储
type InMemoryCSRFStore struct {
	mu     sync.RWMutex
	tokens map[string]csrfTokenEntry
}

type csrfTokenEntry struct {
	token     string
	expiresAt time.Time
}

// NewInMemoryCSRFStore 创建内存 CSRF 存储
func NewInMemoryCSRFStore() *InMemoryCSRFStore {
	store := &InMemoryCSRFStore{
		tokens: make(map[string]csrfTokenEntry),
	}
	// 定期清理过期 token
	go store.cleanupLoop()
	return store
}

// GenerateToken 为会话生成新的 CSRF token
func (s *InMemoryCSRFStore) GenerateToken(sessionID string) (string, error) {
	tokenBytes := make([]byte, csrfTokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[sessionID] = csrfTokenEntry{
		token:     token,
		expiresAt: time.Now().Add(csrfTokenTTL),
	}
	return token, nil
}

// ValidateToken 验证 CSRF token 是否有效
func (s *InMemoryCSRFStore) ValidateToken(sessionID, token string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.tokens[sessionID]
	if !exists {
		return false
	}
	if time.Now().After(entry.expiresAt) {
		return false
	}
	return entry.token == token
}

// cleanupLoop 定期清理过期 token
func (s *InMemoryCSRFStore) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		s.cleanup()
	}
}

// cleanup 清理过期 token
func (s *InMemoryCSRFStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for sessionID, entry := range s.tokens {
		if now.After(entry.expiresAt) {
			delete(s.tokens, sessionID)
		}
	}
}

// SetCSRFCookie 设置 CSRF token cookie
func (api *API) SetCSRFCookie(w http.ResponseWriter, token string) {
	cookie := &http.Cookie{
		Name:     csrfTokenCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false, // JavaScript 需要读取来设置请求头
		Secure:   false, // TODO: 根据配置决定是否启用
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(csrfTokenTTL.Seconds()),
	}
	http.SetCookie(w, cookie)
}

// GetCSRFToken 从请求中获取 CSRF token
func (api *API) GetCSRFToken(r *http.Request) string {
	// 优先从请求头获取
	if token := r.Header.Get(csrfTokenHeaderName); token != "" {
		return token
	}
	// 从 cookie 获取
	if cookie, err := r.Cookie(csrfTokenCookieName); err == nil {
		return cookie.Value
	}
	return ""
}

// ValidateCSRF 验证 CSRF token
func (api *API) ValidateCSRF(r *http.Request, sessionID string) bool {
	token := api.GetCSRFToken(r)
	if token == "" {
		return false
	}
	return api.csrfStore.ValidateToken(sessionID, token)
}

// CSRFProtectionMiddleware 为需要 CSRF 保护的路由添加中间件
func (api *API) CSRFProtectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET, HEAD, OPTIONS 不需要 CSRF 保护
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// 获取会话 ID
		u, err := api.auth.CurrentUser(r)
		if err != nil || u == nil {
			api.Error(w, r, http.StatusUnauthorized, 40101, "unauthorized",
				&ErrorDetail{Type: "auth_required"})
			return
		}

		user, ok := u.(inter.SessionUser)
		if !ok {
			api.InternalError(w, r, 50001, err)
			return
		}

		sessionID := user.GetUsername() // 使用用户名作为会话标识

		// 验证 CSRF token
		if !api.ValidateCSRF(r, sessionID) {
			api.Error(w, r, http.StatusForbidden, 40304, "invalid csrf token",
				&ErrorDetail{Type: "csrf_validation_failed"})
			return
		}

		next.ServeHTTP(w, r)
	})
}
