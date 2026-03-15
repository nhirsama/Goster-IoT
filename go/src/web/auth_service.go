package web

import (
	"context"
	"errors"
	"net/http"

	"github.com/aarondl/authboss/v3"
)

// AuthService 抽象认证依赖，避免 handler 与具体认证库强耦合。
type AuthService interface {
	LoadClientStateMiddleware(next http.Handler) http.Handler
	CurrentUser(r *http.Request) (authboss.User, error)
	NewAuthableUser(ctx context.Context) (authboss.AuthableUser, error)
	CreateUser(ctx context.Context, user authboss.User) error
	HashPassword(password string) (string, error)
	LoadUser(ctx context.Context, pid string) (authboss.User, error)
	VerifyPassword(user authboss.AuthableUser, password string) error
	FireBefore(event authboss.Event, w http.ResponseWriter, r *http.Request) (bool, error)
	FireAfter(event authboss.Event, w http.ResponseWriter, r *http.Request) (bool, error)
	ClearRememberTokens(ctx context.Context, pid string) error
}

type authbossService struct {
	ab *authboss.Authboss
}

func NewAuthService(ab *authboss.Authboss) (AuthService, error) {
	if ab == nil {
		return nil, errors.New("auth service requires authboss instance")
	}
	return &authbossService{ab: ab}, nil
}

func (s *authbossService) LoadClientStateMiddleware(next http.Handler) http.Handler {
	return s.ab.LoadClientStateMiddleware(next)
}

func (s *authbossService) CurrentUser(r *http.Request) (authboss.User, error) {
	return s.ab.CurrentUser(r)
}

func (s *authbossService) NewAuthableUser(ctx context.Context) (authboss.AuthableUser, error) {
	storer, ok := s.ab.Config.Storage.Server.(authboss.CreatingServerStorer)
	if !ok {
		return nil, errors.New("server storer does not support user creation")
	}
	return authboss.MustBeAuthable(storer.New(ctx)), nil
}

func (s *authbossService) CreateUser(ctx context.Context, user authboss.User) error {
	storer, ok := s.ab.Config.Storage.Server.(authboss.CreatingServerStorer)
	if !ok {
		return errors.New("server storer does not support user creation")
	}
	return storer.Create(ctx, user)
}

func (s *authbossService) HashPassword(password string) (string, error) {
	return s.ab.Config.Core.Hasher.GenerateHash(password)
}

func (s *authbossService) LoadUser(ctx context.Context, pid string) (authboss.User, error) {
	return s.ab.Storage.Server.Load(ctx, pid)
}

func (s *authbossService) VerifyPassword(user authboss.AuthableUser, password string) error {
	return s.ab.VerifyPassword(user, password)
}

func (s *authbossService) FireBefore(event authboss.Event, w http.ResponseWriter, r *http.Request) (bool, error) {
	return s.ab.Events.FireBefore(event, w, r)
}

func (s *authbossService) FireAfter(event authboss.Event, w http.ResponseWriter, r *http.Request) (bool, error) {
	return s.ab.Events.FireAfter(event, w, r)
}

func (s *authbossService) ClearRememberTokens(ctx context.Context, pid string) error {
	rememberStorer, ok := s.ab.Config.Storage.Server.(authboss.RememberingServerStorer)
	if !ok {
		return nil
	}
	return rememberStorer.DelRememberTokens(ctx, pid)
}
