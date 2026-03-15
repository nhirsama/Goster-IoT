package inter

import (
	"context"
	"time"
)

// Logger 定义后端统一日志抽象，业务层不直接依赖具体实现。
type Logger interface {
	Debug(msg string, fields ...LogField)
	Info(msg string, fields ...LogField)
	Warn(msg string, fields ...LogField)
	Error(msg string, fields ...LogField)

	DebugContext(ctx context.Context, msg string, fields ...LogField)
	InfoContext(ctx context.Context, msg string, fields ...LogField)
	WarnContext(ctx context.Context, msg string, fields ...LogField)
	ErrorContext(ctx context.Context, msg string, fields ...LogField)

	With(fields ...LogField) Logger
	WithGroup(name string) Logger
}

// LogField 表示一条结构化日志字段。
type LogField struct {
	Key   string
	Value any
}

func String(key, val string) LogField { return LogField{Key: key, Value: val} }

func Int(key string, val int) LogField { return LogField{Key: key, Value: val} }

func Int64(key string, val int64) LogField { return LogField{Key: key, Value: val} }

func Bool(key string, val bool) LogField { return LogField{Key: key, Value: val} }

func Any(key string, val any) LogField { return LogField{Key: key, Value: val} }

func Time(key string, val time.Time) LogField { return LogField{Key: key, Value: val} }

func Duration(key string, val time.Duration) LogField { return LogField{Key: key, Value: val} }

func Err(err error) LogField { return LogField{Key: "err", Value: err} }
