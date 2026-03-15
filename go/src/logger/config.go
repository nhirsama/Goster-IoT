package logger

import (
	"os"
	"strconv"
	"strings"
)

// Config 定义日志组件初始化参数。
type Config struct {
	Level     string
	Format    string
	AddSource bool
	Service   string
	Env       string
}

func defaultConfig() Config {
	return Config{
		Level:     "info",
		Format:    "text",
		AddSource: false,
		Service:   "goster-iot",
		Env:       "dev",
	}
}

// ConfigFromEnv 从环境变量加载日志配置。
func ConfigFromEnv() Config {
	cfg := defaultConfig()

	if raw := strings.TrimSpace(os.Getenv("LOG_LEVEL")); raw != "" {
		cfg.Level = strings.ToLower(raw)
	}
	if raw := strings.TrimSpace(os.Getenv("LOG_FORMAT")); raw != "" {
		cfg.Format = strings.ToLower(raw)
	}
	if raw := strings.TrimSpace(os.Getenv("LOG_ADD_SOURCE")); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			cfg.AddSource = parsed
		}
	}
	if raw := strings.TrimSpace(os.Getenv("LOG_SERVICE")); raw != "" {
		cfg.Service = raw
	}
	if raw := strings.TrimSpace(os.Getenv("LOG_ENV")); raw != "" {
		cfg.Env = raw
	} else if raw := strings.TrimSpace(os.Getenv("APP_ENV")); raw != "" {
		cfg.Env = raw
	}

	return normalizeConfig(cfg)
}

func normalizeConfig(cfg Config) Config {
	base := defaultConfig()
	out := cfg

	switch strings.ToLower(strings.TrimSpace(cfg.Level)) {
	case "debug", "info", "warn", "error":
		out.Level = strings.ToLower(strings.TrimSpace(cfg.Level))
	default:
		out.Level = base.Level
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "json", "text":
		out.Format = strings.ToLower(strings.TrimSpace(cfg.Format))
	default:
		out.Format = base.Format
	}

	if strings.TrimSpace(cfg.Service) == "" {
		out.Service = base.Service
	}
	if strings.TrimSpace(cfg.Env) == "" {
		out.Env = base.Env
	}

	return out
}
