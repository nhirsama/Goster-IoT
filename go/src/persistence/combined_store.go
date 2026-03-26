package persistence

import (
	"context"

	"github.com/aarondl/authboss/v3"
	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	identitycore "github.com/nhirsama/Goster-IoT/src/identity"
)

// Store 组合运行时仓储和认证仓储。
// 它主要用于测试和需要单库便捷访问的开发场景，底层仍然直接走 runtime/auth 两条新链路。
type Store struct {
	RuntimeStore
	auth identitycore.Store
}

var _ identitycore.Store = (*Store)(nil)

// OpenStore 打开组合存储。
func OpenStore(cfg appcfg.DBConfig) (*Store, error) {
	cfg = appcfg.NormalizeDBConfig(cfg)

	if cfg.SchemaMode != "managed" {
		if err := EnsureSchema(cfg); err != nil {
			return nil, err
		}
		cfg.SchemaMode = "managed"
	}

	runtimeStore, err := OpenRuntimeStore(cfg)
	if err != nil {
		return nil, err
	}
	authStore, err := OpenAuthStore(cfg)
	if err != nil {
		_ = CloseIfPossible(runtimeStore)
		return nil, err
	}

	return &Store{
		RuntimeStore: runtimeStore,
		auth:         authStore,
	}, nil
}

// OpenSQLite 是 sqlite 的便捷组合入口。
func OpenSQLite(path string) (*Store, error) {
	return OpenStore(appcfg.DBConfig{
		Driver: "sqlite",
		Path:   path,
	})
}

// OpenPostgres 是 postgres 的便捷组合入口。
func OpenPostgres(dsn string) (*Store, error) {
	return OpenStore(appcfg.DBConfig{
		Driver: "postgres",
		DSN:    dsn,
	})
}

func (s *Store) Close() error {
	var firstErr error
	if err := CloseIfPossible(s.auth); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := CloseIfPossible(s.RuntimeStore); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (s *Store) Load(ctx context.Context, key string) (authboss.User, error) {
	return s.auth.Load(ctx, key)
}

func (s *Store) Save(ctx context.Context, user authboss.User) error {
	return s.auth.Save(ctx, user)
}

func (s *Store) New(ctx context.Context) authboss.User {
	return s.auth.New(ctx)
}

func (s *Store) Create(ctx context.Context, user authboss.User) error {
	return s.auth.Create(ctx, user)
}

func (s *Store) NewFromOAuth2(ctx context.Context, provider string, details map[string]string) (authboss.OAuth2User, error) {
	return s.auth.NewFromOAuth2(ctx, provider, details)
}

func (s *Store) SaveOAuth2(ctx context.Context, user authboss.OAuth2User) error {
	return s.auth.SaveOAuth2(ctx, user)
}

func (s *Store) AddRememberToken(ctx context.Context, pid, token string) error {
	return s.auth.AddRememberToken(ctx, pid, token)
}

func (s *Store) DelRememberTokens(ctx context.Context, pid string) error {
	return s.auth.DelRememberTokens(ctx, pid)
}

func (s *Store) UseRememberToken(ctx context.Context, pid, token string) error {
	return s.auth.UseRememberToken(ctx, pid, token)
}
