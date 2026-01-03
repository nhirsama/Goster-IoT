package Web

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/dchest/captcha"
)

// CaptchaProvider 定义验证码策略接口
type CaptchaProvider interface {
	// GetTemplateData 返回渲染验证码所需的数据 (例如 CaptchaId 或 SiteKey)
	GetTemplateData() map[string]interface{}
	// Verify 验证请求中的验证码
	Verify(r *http.Request) bool
	// Type 返回 "local" 或 "turnstile"
	Type() string
}

// LocalCaptcha 实现 dchest/captcha 策略
type LocalCaptcha struct{}

func (l *LocalCaptcha) GetTemplateData() map[string]interface{} {
	return map[string]interface{}{
		"CaptchaType": "local",
		"CaptchaId":   captcha.New(),
	}
}

func (l *LocalCaptcha) Verify(r *http.Request) bool {
	return captcha.VerifyString(r.FormValue("captchaId"), r.FormValue("captchaSolution"))
}

func (l *LocalCaptcha) Type() string {
	return "local"
}

// CloudflareTurnstile 实现 Cloudflare Turnstile 策略
type CloudflareTurnstile struct {
	SiteKey   string
	SecretKey string
}

func (c *CloudflareTurnstile) GetTemplateData() map[string]interface{} {
	return map[string]interface{}{
		"CaptchaType": "turnstile",
		"SiteKey":     c.SiteKey,
	}
}

type turnstileResponse struct {
	Success bool `json:"success"`
}

func (c *CloudflareTurnstile) Verify(r *http.Request) bool {
	token := r.FormValue("cf-turnstile-response")
	ip := r.RemoteAddr

	resp, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", map[string][]string{
		"secret":   {c.SecretKey},
		"response": {token},
		"remoteip": {ip},
	})
	if err != nil {
		log.Printf("Turnstile 验证失败: %v", err)
		return false
	}
	defer resp.Body.Close()

	var result turnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	return result.Success
}

func (c *CloudflareTurnstile) Type() string {
	return "turnstile"
}
