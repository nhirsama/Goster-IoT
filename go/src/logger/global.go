package logger

import (
	"sync"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

var (
	defaultLoggerMu sync.RWMutex
	defaultLogger   inter.Logger = NewNoop()
)

// SetDefault 设置全局默认日志器，供过渡期代码使用。
func SetDefault(l inter.Logger) {
	if l == nil {
		l = NewNoop()
	}
	defaultLoggerMu.Lock()
	defaultLogger = l
	defaultLoggerMu.Unlock()
}

// Default 返回全局默认日志器。
func Default() inter.Logger {
	defaultLoggerMu.RLock()
	l := defaultLogger
	defaultLoggerMu.RUnlock()
	return l
}
