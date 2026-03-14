package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// TurnstileService 管理 Cloudflare Turnstile 验证
type TurnstileService struct {
	SiteKey   string
	SecretKey string
	Enabled   bool
	client    *http.Client
}

const turnstileVerifyTimeout = 5 * time.Second

// NewTurnstileService 初始化服务
func NewTurnstileService() *TurnstileService {
	provider := os.Getenv("CAPTCHA_PROVIDER")
	return &TurnstileService{
		SiteKey:   os.Getenv("CF_SITE_KEY"),
		SecretKey: os.Getenv("CF_SECRET_KEY"),
		Enabled:   provider == "turnstile",
		client: &http.Client{
			Timeout: turnstileVerifyTimeout,
		},
	}
}

func (s *TurnstileService) IsEnabled() bool {
	return s != nil && s.Enabled
}

func (s *TurnstileService) PublicSiteKey() string {
	if s == nil {
		return ""
	}
	return s.SiteKey
}

type turnstileResponse struct {
	Success bool `json:"success"`
}

// Verify 验证请求中的 Token
func (s *TurnstileService) Verify(r *http.Request) bool {
	if !s.Enabled {
		return true
	}

	token := r.FormValue("cf-turnstile-response")
	if token == "" {
		log.Println("Turnstile 验证失败: 提交的 cf-turnstile-response 为空")
		return false
	}

	return s.VerifyToken(token, clientIPFromRequest(r))
}

// VerifyToken 使用给定 token 和客户端 IP 进行验证
func (s *TurnstileService) VerifyToken(token string, ip string) bool {
	if !s.Enabled {
		return true
	}
	if strings.TrimSpace(token) == "" {
		log.Println("Turnstile 验证失败: token 为空")
		return false
	}

	form := url.Values{
		"secret":   {s.SecretKey},
		"response": {token},
		"remoteip": {ip},
	}
	ctx, cancel := context.WithTimeout(context.Background(), turnstileVerifyTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(form.Encode()))
	if err != nil {
		log.Printf("Turnstile 请求构建失败: %v", err)
		return false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := s.client
	if client == nil {
		client = &http.Client{Timeout: turnstileVerifyTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Turnstile 验证请求失败 (网络错误): %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Turnstile 验证失败: 非 200 状态码 %d", resp.StatusCode)
		return false
	}

	var result turnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Turnstile 响应解析失败: %v", err)
		return false
	}

	if !result.Success {
		log.Println("Turnstile 验证失败: 验证码服务返回 Success=false")
	}
	return result.Success
}

func clientIPFromRequest(r *http.Request) string {
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	return r.RemoteAddr
}
