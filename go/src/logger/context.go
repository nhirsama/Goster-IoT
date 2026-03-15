package logger

import (
	"context"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

type ctxLoggerKey struct{}

// IntoContext 将日志器写入上下文。
func IntoContext(ctx context.Context, l inter.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if l == nil {
		l = Default()
	}
	return context.WithValue(ctx, ctxLoggerKey{}, l)
}

// FromContext 从上下文读取日志器，缺失时返回默认日志器。
func FromContext(ctx context.Context) inter.Logger {
	if ctx == nil {
		return Default()
	}
	if l, ok := ctx.Value(ctxLoggerKey{}).(inter.Logger); ok && l != nil {
		return l
	}
	return Default()
}
