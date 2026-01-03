package Web

import (
	"errors"
	"net/http"
	"os"

	"context"
	"encoding/json"
	"strconv"

	"fmt"

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

// SetupAuthboss initializes and configures the Authboss instance
func SetupAuthboss(db inter.DataStore, htmlDir string) (*authboss.Authboss, error) {
	ab := authboss.New()

	// Ensure DataStore implements Authboss ServerStorer
	storer, ok := db.(authboss.ServerStorer)
	if !ok {
		return nil, errors.New("DataStore does not implement Authboss ServerStorer")
	}
	ab.Config.Storage.Server = storer

	// Session Store (MaxAge = 0, deleted on browser close)
	sessionStore := sessions.NewCookieStore([]byte("super-secret-key-change-me"))
	sessionStore.Options.MaxAge = 0
	sessionStore.Options.HttpOnly = true
	sessionStore.Options.Secure = false // Localhost dev
	sessionStore.Options.Path = "/"
	sessionStore.Options.SameSite = http.SameSiteLaxMode
	ab.Config.Storage.SessionState = NewSessionStorer("goster_session", sessionStore)

	// Cookie Store (Remember Me, MaxAge = 30 days)
	cookieStore := sessions.NewCookieStore([]byte("super-secret-key-change-me"))
	cookieStore.Options.MaxAge = 86400 * 30
	cookieStore.Options.HttpOnly = true
	cookieStore.Options.Secure = false // Localhost dev
	cookieStore.Options.Path = "/"
	cookieStore.Options.SameSite = http.SameSiteLaxMode
	ab.Config.Storage.CookieState = NewSessionStorer("goster_remember", cookieStore)

	ab.Config.Paths.Mount = "/auth"
	ab.Config.Paths.RootURL = "http://localhost:8080"
	ab.Config.Paths.RegisterOK = "/auth/login"
	ab.Config.Paths.LogoutOK = "/auth/login"
	ab.Config.Paths.OAuth2LoginOK = "/"

	// Basic defaults (No confirm, No lock)
	// Enable UseUsername (3rd arg = true) for BodyReader
	defaults.SetCore(&ab.Config, false, true)

	// Allow GET for logout (convenient for links)
	ab.Config.Modules.LogoutMethod = "GET"

	// OAuth2 Providers
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
					fmt.Printf("GitHub API Get Error: %v\n", err)
					return nil, err
				}
				defer resp.Body.Close()

				var ghUser struct {
					ID    int    `json:"id"`
					Email string `json:"email"`
					Login string `json:"login"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
					fmt.Printf("GitHub API Decode Error: %v\n", err)
					return nil, err
				}

				fmt.Printf("GitHub User Fetched: ID=%d, Email=%s, Login=%s\n", ghUser.ID, ghUser.Email, ghUser.Login)

				return map[string]string{
					aboauth2.OAuth2UID:   strconv.Itoa(ghUser.ID),
					aboauth2.OAuth2Email: ghUser.Email,
				}, nil
			},
		},
	}

	// Set HTML Renderer
	renderer := NewHTMLRenderer(htmlDir)
	ab.Config.Core.ViewRenderer = renderer
	// Re-initialize Responder with the correct renderer
	ab.Config.Core.Responder = defaults.NewResponder(renderer)

	if err := ab.Init(); err != nil {
		return nil, err
	}

	return ab, nil
}
