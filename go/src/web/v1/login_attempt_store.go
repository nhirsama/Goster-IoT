package v1

import "time"

// LoginAttemptSnapshot 表示某个登录尝试键当前的失败历史和锁定状态。
type LoginAttemptSnapshot struct {
	Failures    []time.Time
	LockedUntil time.Time
}

// LoginAttemptStore 抽象登录失败状态的存储语义。
// 当前默认实现仍然是内存版，但接口边界允许后续替换成 Redis 等共享存储。
type LoginAttemptStore interface {
	Snapshot(key string, now time.Time, window time.Duration) (LoginAttemptSnapshot, error)
	RecordFailure(key string, now time.Time, window, lockout time.Duration, maxFailures int) error
	Reset(key string) error
}
