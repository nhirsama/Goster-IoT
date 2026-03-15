package logger

import (
	"context"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

type noopLogger struct{}

// NewNoop 返回一个空实现，适用于测试和兜底场景。
func NewNoop() inter.Logger {
	return noopLogger{}
}

func (noopLogger) Debug(_ string, _ ...inter.LogField) {}

func (noopLogger) Info(_ string, _ ...inter.LogField) {}

func (noopLogger) Warn(_ string, _ ...inter.LogField) {}

func (noopLogger) Error(_ string, _ ...inter.LogField) {}

func (noopLogger) DebugContext(_ context.Context, _ string, _ ...inter.LogField) {}

func (noopLogger) InfoContext(_ context.Context, _ string, _ ...inter.LogField) {}

func (noopLogger) WarnContext(_ context.Context, _ string, _ ...inter.LogField) {}

func (noopLogger) ErrorContext(_ context.Context, _ string, _ ...inter.LogField) {}

func (n noopLogger) With(_ ...inter.LogField) inter.Logger {
	return n
}

func (n noopLogger) WithGroup(_ string) inter.Logger {
	return n
}
