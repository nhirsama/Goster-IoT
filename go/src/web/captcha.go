package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
)

// TurnstileService 管理 Cloudflare Turnstile 验证
type TurnstileService struct {
	SiteKey   string
	SecretKey string
	Enabled   bool
	client    *http.Client
	timeout   time.Duration
	logger    inter.Logger
}

// NewTurnstileService 初始化服务
func NewTurnstileService() *TurnstileService {
	loaded, err := appcfg.Load()
	if err != nil {
		return NewTurnstileServiceWithConfig(appcfg.DefaultCaptchaConfig())
	}
	return NewTurnstileServiceWithConfig(loaded.Captcha)
}

func NewTurnstileServiceWithConfig(cfg appcfg.CaptchaConfig) *TurnstileService {
	cfg = appcfg.NormalizeCaptchaConfig(cfg)
	timeout := cfg.VerifyTimeout
	return &TurnstileService{
		SiteKey:   cfg.SiteKey,
		SecretKey: cfg.SecretKey,
		Enabled:   strings.EqualFold(strings.TrimSpace(cfg.Provider), "turnstile"),
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		logger:  logger.Default().With(inter.String("module", "captcha")),
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
		s.log().Warn("Turnstile 校验失败：表单响应为空")
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
		s.log().Warn("Turnstile 校验失败：token 为空")
		return false
	}

	form := url.Values{
		"secret":   {s.SecretKey},
		"response": {token},
		"remoteip": {ip},
	}
	timeout := s.timeout
	if timeout <= 0 {
		timeout = appcfg.DefaultCaptchaConfig().VerifyTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(form.Encode()))
	if err != nil {
		s.log().Warn("Turnstile 请求构建失败", inter.Err(err))
		return false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := s.client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		s.log().Warn("Turnstile 请求失败", inter.Err(err))
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.log().Warn("Turnstile 校验失败：状态码非 200", inter.Int("status_code", resp.StatusCode))
		return false
	}

	var result turnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		s.log().Warn("Turnstile 响应解析失败", inter.Err(err))
		return false
	}

	if !result.Success {
		s.log().Warn("Turnstile 校验失败：返回 success=false")
	}
	return result.Success
}

func (s *TurnstileService) log() inter.Logger {
	if s != nil && s.logger != nil {
		return s.logger
	}
	return logger.Default().With(inter.String("module", "captcha"))
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
