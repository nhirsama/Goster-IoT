package identity

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/aarondl/authboss/v3"
	_ "github.com/aarondl/authboss/v3/auth"
	"github.com/aarondl/authboss/v3/defaults"
	_ "github.com/aarondl/authboss/v3/logout"
	aboauth2 "github.com/aarondl/authboss/v3/oauth2"
	_ "github.com/aarondl/authboss/v3/register"
	_ "github.com/aarondl/authboss/v3/remember"
	"github.com/gorilla/sessions"
	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

func generateRandomKey(length int) []byte {
	k := make([]byte, length)
	if _, err := rand.Read(k); err != nil {
		panic("failed to generate random key: " + err.Error())
	}
	return k
}

// SetupAuthboss 使用全局配置初始化 Authboss。
func SetupAuthboss(db inter.DataStore) (*authboss.Authboss, error) {
	loaded, err := appcfg.Load()
	if err != nil {
		return nil, err
	}
	return SetupAuthbossWithConfig(db, loaded.Auth)
}

// SetupAuthbossWithConfig 初始化并配置 Authboss 实例。
// 这一层只负责认证流程，不承担租户授权和租户上下文管理。
func SetupAuthbossWithConfig(db inter.DataStore, cfg appcfg.AuthConfig) (*authboss.Authboss, error) {
	ab := authboss.New()
	cfg = appcfg.NormalizeAuthConfig(cfg)
	cookieSecure := cfg.CookieSecure

	storer, ok := db.(authboss.ServerStorer)
	if !ok {
		return nil, errors.New("datastore 未实现 Authboss ServerStorer 接口")
	}
	ab.Config.Storage.Server = storer

	sessionKey := generateRandomKey(64)
	sessionStore := sessions.NewCookieStore(sessionKey)
	sessionStore.Options.MaxAge = cfg.SessionCookieMaxAgeSeconds
	sessionStore.Options.HttpOnly = true
	sessionStore.Options.Secure = cookieSecure
	sessionStore.Options.Path = "/"
	sessionStore.Options.SameSite = http.SameSiteLaxMode
	ab.Config.Storage.SessionState = NewSessionStorer("goster_session", sessionStore)

	cookieKey := generateRandomKey(64)
	cookieStore := sessions.NewCookieStore(cookieKey)
	cookieStore.Options.MaxAge = cfg.RememberCookieMaxAgeSeconds
	cookieStore.Options.HttpOnly = true
	cookieStore.Options.Secure = cookieSecure
	cookieStore.Options.Path = "/"
	cookieStore.Options.SameSite = http.SameSiteLaxMode
	ab.Config.Storage.CookieState = NewSessionStorer("goster_remember", cookieStore)

	ab.Config.Paths.Mount = "/auth"
	ab.Config.Paths.RootURL = strings.TrimSpace(cfg.RootURL)
	if ab.Config.Paths.RootURL == "" {
		ab.Config.Paths.RootURL = appcfg.DefaultAuthConfig().RootURL
	}

	ab.Config.Paths.RegisterOK = "/login"
	ab.Config.Paths.LogoutOK = "/login"
	ab.Config.Paths.OAuth2LoginOK = "/"

	defaults.SetCore(&ab.Config, false, true)
	ab.Config.Modules.LogoutMethod = "GET"

	ab.Config.Modules.OAuth2Providers = map[string]authboss.OAuth2Provider{
		"github": {
			OAuth2Config: &oauth2.Config{
				ClientID:     strings.TrimSpace(cfg.GitHubClientID),
				ClientSecret: strings.TrimSpace(cfg.GitHubClientSecret),
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

	renderer := NewStaticViewRenderer()
	ab.Config.Core.ViewRenderer = renderer
	ab.Config.Core.Responder = defaults.NewResponder(renderer)

	if err := ab.Init(); err != nil {
		return nil, err
	}

	return ab, nil
}
