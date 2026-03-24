package v1

import (
	"math"
	"net"
	"strings"
	"sync"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
)

// LoginAttemptGuard 在进程内维护一个轻量的登录失败锁定窗口。
type LoginAttemptGuard struct {
	mu          sync.Mutex
	maxFailures int
	window      time.Duration
	lockout     time.Duration
	attempts    map[string]*loginAttemptState
	now         func() time.Time
}

type loginAttemptState struct {
	failures    []time.Time
	lockedUntil time.Time
}

func newLoginAttemptGuard(cfg appcfg.LoginProtectionConfig) *LoginAttemptGuard {
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
	return &LoginAttemptGuard{
		maxFailures: cfg.MaxFailures,
		window:      cfg.Window,
		lockout:     cfg.Lockout,
		attempts:    make(map[string]*loginAttemptState),
		now:         time.Now,
	}
}

// NewLoginAttemptGuard 根据标准化后的安全配置创建登录保护器。
func NewLoginAttemptGuard(cfg appcfg.LoginProtectionConfig) *LoginAttemptGuard {
	return newLoginAttemptGuard(cfg)
}

// SetClockForTest 允许测试注入时钟，从而避免依赖真实时间等待。
func (g *LoginAttemptGuard) SetClockForTest(now func() time.Time) {
	if g != nil && now != nil {
		g.now = now
	}
}

func (g *LoginAttemptGuard) Allow(username, remoteAddr string) (time.Duration, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	state := g.stateFor(username, remoteAddr)
	now := g.now()
	g.pruneFailures(now, state)
	if state.lockedUntil.After(now) {
		return state.lockedUntil.Sub(now), false
	}
	g.cleanup(username, remoteAddr, state)
	return 0, true
}

func (g *LoginAttemptGuard) RecordFailure(username, remoteAddr string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	state := g.stateFor(username, remoteAddr)
	now := g.now()
	g.pruneFailures(now, state)
	state.failures = append(state.failures, now)
	if len(state.failures) >= g.maxFailures {
		state.lockedUntil = now.Add(g.lockout)
	}
}

func (g *LoginAttemptGuard) Reset(username, remoteAddr string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.attempts, loginAttemptKey(username, remoteAddr))
}

func (g *LoginAttemptGuard) stateFor(username, remoteAddr string) *loginAttemptState {
	key := loginAttemptKey(username, remoteAddr)
	state, ok := g.attempts[key]
	if !ok {
		state = &loginAttemptState{}
		g.attempts[key] = state
	}
	return state
}

func (g *LoginAttemptGuard) pruneFailures(now time.Time, state *loginAttemptState) {
	cutoff := now.Add(-g.window)
	kept := state.failures[:0]
	for _, ts := range state.failures {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	state.failures = kept
	if !state.lockedUntil.After(now) {
		state.lockedUntil = time.Time{}
	}
}

func (g *LoginAttemptGuard) cleanup(username, remoteAddr string, state *loginAttemptState) {
	if len(state.failures) == 0 && state.lockedUntil.IsZero() {
		delete(g.attempts, loginAttemptKey(username, remoteAddr))
	}
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
