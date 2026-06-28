package identity

import (
	"context"
	"errors"
	"net/http"

	"github.com/aarondl/authboss/v3"
)

// Service 抽象认证能力，隔离 web handler 与 Authboss 的直接耦合。
type Service interface {
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

// NewAuthbossService 构造基于 Authboss 的认证服务适配器。
func NewAuthbossService(ab *authboss.Authboss) (Service, error) {
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
