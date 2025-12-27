package Web

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/dchest/captcha"
	"github.com/nhirsama/Goster-IoT/src/inter"
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

// loginHandler 登录处理
func (ws *webServer) loginHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")

	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		perm, err := ws.dataStore.LoginUser(username, password)
		if err != nil {
			ws.templates["login.html"].Execute(w, map[string]interface{}{"Error": "用户名或密码错误"})
			return
		}

		session.Values["authenticated"] = true
		session.Values["username"] = username
		session.Values["permission"] = int(perm)
		session.Save(r, w)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	ws.templates["login.html"].Execute(w, nil)
}

// registerHandler 注册处理
func (ws *webServer) registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if !ws.captcha.Verify(r) {
			data := ws.captcha.GetTemplateData()
			data["Error"] = "验证码错误"
			ws.templates["register.html"].Execute(w, data)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		count, err := ws.dataStore.GetUserCount()
		if err != nil {
			http.Error(w, "数据库错误", http.StatusInternalServerError)
			return
		}

		var perm inter.PermissionType
		if count == 0 {
			perm = inter.PermissionAdmin
		} else {
			perm = inter.PermissionNone // 默认为无权限
		}

		err = ws.dataStore.RegisterUser(username, password, perm)
		if err != nil {
			data := ws.captcha.GetTemplateData()
			data["Error"] = "注册失败: " + err.Error()
			ws.templates["register.html"].Execute(w, data)
			return
		}

		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if t, ok := ws.templates["register.html"]; ok {
		t.Execute(w, ws.captcha.GetTemplateData())
	} else {
		http.Error(w, "注册模板丢失", http.StatusInternalServerError)
	}
}

// logoutHandler 登出处理
func (ws *webServer) logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	session.Values["authenticated"] = false
	session.Values["username"] = ""
	session.Values["permission"] = 0
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusFound)
}
