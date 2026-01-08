package web

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"

	"github.com/aarondl/authboss/v3"
	_ "github.com/aarondl/authboss/v3/auth"
	"github.com/aarondl/authboss/v3/defaults"
	_ "github.com/aarondl/authboss/v3/logout"
	aboauth2 "github.com/aarondl/authboss/v3/oauth2"
	_ "github.com/aarondl/authboss/v3/register"
	_ "github.com/aarondl/authboss/v3/remember"
	"github.com/gorilla/sessions"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// generateRandomKey 生成指定长度的随机字节切片
func generateRandomKey(length int) []byte {
	k := make([]byte, length)
	if _, err := rand.Read(k); err != nil {
		// 如果随机源失败，panic 是合理的，因为无法保证安全
		panic("failed to generate random key: " + err.Error())
	}
	return k
}

// SetupAuthboss 初始化并配置 Authboss 实例
func SetupAuthboss(db inter.DataStore, htmlDir string) (*authboss.Authboss, error) {
	ab := authboss.New()

	// 确保 datastore 实现了 Authboss ServerStorer
	storer, ok := db.(authboss.ServerStorer)
	if !ok {
		return nil, errors.New("datastore 未实现 Authboss ServerStorer 接口")
	}
	ab.Config.Storage.Server = storer

	// Session Store (MaxAge = 0, 浏览器关闭即删除)
	// 使用随机密钥：重启后 Session 失效，但安全性更高
	sessionKey := generateRandomKey(64)
	sessionStore := sessions.NewCookieStore(sessionKey)
	sessionStore.Options.MaxAge = 0
	sessionStore.Options.HttpOnly = true
	sessionStore.Options.Secure = false // 本地开发环境设为 false
	sessionStore.Options.Path = "/"
	sessionStore.Options.SameSite = http.SameSiteLaxMode
	ab.Config.Storage.SessionState = NewSessionStorer("goster_session", sessionStore)

	// Cookie Store (Remember Me, 有效期 30 天)
	// 使用独立的随机密钥
	cookieKey := generateRandomKey(64)
	cookieStore := sessions.NewCookieStore(cookieKey)
	cookieStore.Options.MaxAge = 86400 * 30
	cookieStore.Options.HttpOnly = true
	cookieStore.Options.Secure = false // 本地开发环境设为 false
	cookieStore.Options.Path = "/"
	cookieStore.Options.SameSite = http.SameSiteLaxMode
	ab.Config.Storage.CookieState = NewSessionStorer("goster_remember", cookieStore)

	ab.Config.Paths.Mount = "/auth"
	ab.Config.Paths.RootURL = os.Getenv("AUTHBOSS_ROOT_URL")
	if ab.Config.Paths.RootURL == "" {
		ab.Config.Paths.RootURL = "http://localhost:8080"
	}

	ab.Config.Paths.RegisterOK = "/auth/login"
	ab.Config.Paths.LogoutOK = "/auth/login"
	ab.Config.Paths.OAuth2LoginOK = "/"

	// 基础默认配置 (无 Confirm, 无 Lock)
	// 启用 UseUsername (第3个参数 = true) 以便 BodyReader 读取 username 字段
	defaults.SetCore(&ab.Config, false, true)

	// 允许 GET 方法注销 (方便链接跳转)
	ab.Config.Modules.LogoutMethod = "GET"

	// OAuth2 提供商配置
	ab.Config.Modules.OAuth2Providers = map[string]authboss.OAuth2Provider{
		"github": {
			OAuth2Config: &oauth2.Config{
				ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
				ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
				Scopes:       []string{"user:email"},
				Endpoint:     github.Endpoint,
			},
			FindUserDetails: func(ctx context.Context, cfg oauth2.Config, token *oauth2.Token) (map[string]string, error) {
				client := cfg.Client(ctx, token)
				resp, err := client.Get("https://api.github.com/user")
				if err != nil {
					return nil, err
				}
				defer resp.Body.Close()

				var ghUser struct {
					ID    int    `json:"id"`
					Email string `json:"email"`
					Login string `json:"login"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
					return nil, err
				}

				return map[string]string{
					aboauth2.OAuth2UID:   strconv.Itoa(ghUser.ID),
					aboauth2.OAuth2Email: ghUser.Email,
				}, nil
			},
		},
	}

	// 设置 HTML 渲染器
	renderer := NewHTMLRenderer(htmlDir)
	ab.Config.Core.ViewRenderer = renderer
	// 使用正确的渲染器重新初始化 Responder
	ab.Config.Core.Responder = defaults.NewResponder(renderer)

	if err := ab.Init(); err != nil {
		return nil, err
	}

	return ab, nil
}
