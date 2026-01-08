package web

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

// TurnstileService 管理 Cloudflare Turnstile 验证
type TurnstileService struct {
	SiteKey   string
	SecretKey string
	Enabled   bool
}

// NewTurnstileService 初始化服务
func NewTurnstileService() *TurnstileService {
	provider := os.Getenv("CAPTCHA_PROVIDER")
	return &TurnstileService{
		SiteKey:   os.Getenv("CF_SITE_KEY"),
		SecretKey: os.Getenv("CF_SECRET_KEY"),
		Enabled:   provider == "turnstile",
	}
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
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}

	resp, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", map[string][]string{
		"secret":   {s.SecretKey},
		"response": {token},
		"remoteip": {ip},
	})
	if err != nil {
		log.Printf("Turnstile 验证请求失败 (网络错误): %v", err)
		return false
	}
	defer resp.Body.Close()

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
