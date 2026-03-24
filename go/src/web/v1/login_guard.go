package v1

import (
	"math"
	"net"
	"strings"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
)

// LoginAttemptGuard 负责登录失败保护策略本身。
// 具体状态落在哪里，由 LoginAttemptStore 决定。
type LoginAttemptGuard struct {
	maxFailures int
	window      time.Duration
	lockout     time.Duration
	store       LoginAttemptStore
	now         func() time.Time
}

func newLoginAttemptGuard(cfg appcfg.LoginProtectionConfig, store LoginAttemptStore) *LoginAttemptGuard {
	defaults := appcfg.DefaultWebConfig().LoginProtection
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = defaults.MaxFailures
	}
	if cfg.Window <= 0 {
		cfg.Window = defaults.Window
	}
	if cfg.Lockout <= 0 {
		cfg.Lockout = defaults.Lockout
	}
	if store == nil {
		store = NewInMemoryLoginAttemptStore()
	}
	return &LoginAttemptGuard{
		maxFailures: cfg.MaxFailures,
		window:      cfg.Window,
		lockout:     cfg.Lockout,
		store:       store,
		now:         time.Now,
	}
}

// NewLoginAttemptGuard 根据标准化后的安全配置创建登录保护器。
func NewLoginAttemptGuard(cfg appcfg.LoginProtectionConfig) *LoginAttemptGuard {
	return newLoginAttemptGuard(cfg, nil)
}

// NewLoginAttemptGuardWithStore 使用指定状态存储创建登录保护器。
func NewLoginAttemptGuardWithStore(cfg appcfg.LoginProtectionConfig, store LoginAttemptStore) *LoginAttemptGuard {
	return newLoginAttemptGuard(cfg, store)
}

// SetClockForTest 允许测试注入时钟，从而避免依赖真实时间等待。
func (g *LoginAttemptGuard) SetClockForTest(now func() time.Time) {
	if g != nil && now != nil {
		g.now = now
	}
}

func (g *LoginAttemptGuard) Allow(username, remoteAddr string) (time.Duration, bool, error) {
	now := g.now()
	state, err := g.store.Snapshot(loginAttemptKey(username, remoteAddr), now, g.window)
	if err != nil {
		return 0, false, err
	}
	if state.LockedUntil.After(now) {
		return state.LockedUntil.Sub(now), false, nil
	}
	return 0, true, nil
}

func (g *LoginAttemptGuard) RecordFailure(username, remoteAddr string) error {
	return g.store.RecordFailure(loginAttemptKey(username, remoteAddr), g.now(), g.window, g.lockout, g.maxFailures)
}

func (g *LoginAttemptGuard) Reset(username, remoteAddr string) error {
	return g.store.Reset(loginAttemptKey(username, remoteAddr))
}

func retryAfterSeconds(d time.Duration) int {
	if d <= 0 {
		return 1
	}
	return int(math.Ceil(d.Seconds()))
}

func loginAttemptKey(username, remoteAddr string) string {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		username = "unknown"
	}
	clientIP := loginAttemptIP(remoteAddr)
	return username + "|" + clientIP
}

func loginAttemptIP(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil && strings.TrimSpace(host) != "" {
		return strings.TrimSpace(host)
	}
	return remoteAddr
}
