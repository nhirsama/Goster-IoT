package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

type slogLogger struct {
	base *slog.Logger
}

// New 创建基于 slog 的日志实现。
func New(cfg Config) inter.Logger {
	return newWithWriter(cfg, os.Stdout)
}

func newWithWriter(cfg Config, out io.Writer) inter.Logger {
	cfg = normalizeConfig(cfg)
	if out == nil {
		out = os.Stdout
	}

	opts := &slog.HandlerOptions{
		AddSource: cfg.AddSource,
		Level:     parseLevel(cfg.Level),
	}

	var h slog.Handler
	if cfg.Format == "json" {
		h = slog.NewJSONHandler(out, opts)
	} else {
		h = slog.NewTextHandler(out, opts)
	}

	l := slog.New(h).With(
		slog.String("service", cfg.Service),
		slog.String("env", cfg.Env),
	)
	return slogLogger{base: l}
}

func (l slogLogger) Debug(msg string, fields ...inter.LogField) {
	l.log(context.Background(), slog.LevelDebug, msg, fields...)
}

func (l slogLogger) Info(msg string, fields ...inter.LogField) {
	l.log(context.Background(), slog.LevelInfo, msg, fields...)
}

func (l slogLogger) Warn(msg string, fields ...inter.LogField) {
	l.log(context.Background(), slog.LevelWarn, msg, fields...)
}

func (l slogLogger) Error(msg string, fields ...inter.LogField) {
	l.log(context.Background(), slog.LevelError, msg, fields...)
}

func (l slogLogger) DebugContext(ctx context.Context, msg string, fields ...inter.LogField) {
	l.log(ctx, slog.LevelDebug, msg, fields...)
}

func (l slogLogger) InfoContext(ctx context.Context, msg string, fields ...inter.LogField) {
	l.log(ctx, slog.LevelInfo, msg, fields...)
}

func (l slogLogger) WarnContext(ctx context.Context, msg string, fields ...inter.LogField) {
	l.log(ctx, slog.LevelWarn, msg, fields...)
}

func (l slogLogger) ErrorContext(ctx context.Context, msg string, fields ...inter.LogField) {
	l.log(ctx, slog.LevelError, msg, fields...)
}

func (l slogLogger) With(fields ...inter.LogField) inter.Logger {
	attrs := toAttrs(RedactFields(fields...))
	if len(attrs) == 0 {
		return l
	}
	return slogLogger{base: l.base.With(attrsToAny(attrs)...)}
}

func (l slogLogger) WithGroup(name string) inter.Logger {
	if strings.TrimSpace(name) == "" {
		return l
	}
	return slogLogger{base: l.base.WithGroup(name)}
}

func (l slogLogger) log(ctx context.Context, level slog.Level, msg string, fields ...inter.LogField) {
	if ctx == nil {
		ctx = context.Background()
	}
	l.base.LogAttrs(ctx, level, msg, toAttrs(RedactFields(fields...))...)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func toAttrs(fields []inter.LogField) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(fields))
	for _, f := range fields {
		attrs = append(attrs, slog.Any(f.Key, f.Value))
	}
	return attrs
}

func attrsToAny(attrs []slog.Attr) []any {
	args := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		args = append(args, attr)
	}
	return args
}
