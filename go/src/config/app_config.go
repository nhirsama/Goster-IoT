package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/src/logger"
	"github.com/spf13/viper"
)

const (
	defaultDBPath                           = "./data.db"
	defaultWebHTTPAddr                      = ":8080"
	defaultAPITCPAddr                       = ":8081"
	defaultAPICORSAllowOrigins              = "http://localhost:5173,http://127.0.0.1:5173"
	defaultAuthRootURL                      = "http://localhost:8080"
	defaultMaxAPIBodyBytes            int64 = 1 << 20
	defaultMetricsMinValidTimestampMs int64 = 1672531200000
	defaultMetricsRangeLabel                = "1h"
)

type AppConfig struct {
	DB            DBConfig
	Web           WebConfig
	API           APIConfig
	Auth          AuthConfig
	Captcha       CaptchaConfig
	DeviceManager DeviceManagerConfig
	Logger        logger.Config
}

type DBConfig struct {
	Path string
}

type WebConfig struct {
	HTTPAddr            string
	APICORSAllowOrigins string
	MaxAPIBodyBytes     int64
	DeviceListPage      PaginationConfig
	Metrics             MetricsConfig
}

type APIConfig struct {
	TCPAddr               string
	ReadTimeout           time.Duration
	RegisterAckGraceDelay time.Duration
}

type AuthConfig struct {
	RootURL                     string
	CookieSecure                bool
	GitHubClientID              string
	GitHubClientSecret          string
	SessionCookieMaxAgeSeconds  int
	RememberCookieMaxAgeSeconds int
}

type CaptchaConfig struct {
	Provider      string
	SiteKey       string
	SecretKey     string
	VerifyTimeout time.Duration
}

type DeviceManagerConfig struct {
	QueueCapacity            int
	HeartbeatDeadline        time.Duration
	ExternalListPage         PaginationConfig
	ExternalObservationLimit LimitConfig
}

type PaginationConfig struct {
	DefaultSize int
	MaxSize     int
}

type LimitConfig struct {
	Default int
	Max     int
}

type MetricsConfig struct {
	MinValidTimestampMs int64
	DefaultRangeLabel   string
}

func DefaultDBConfig() DBConfig {
	return DBConfig{Path: defaultDBPath}
}

func DefaultWebConfig() WebConfig {
	return WebConfig{
		HTTPAddr:            defaultWebHTTPAddr,
		APICORSAllowOrigins: defaultAPICORSAllowOrigins,
		MaxAPIBodyBytes:     defaultMaxAPIBodyBytes,
		DeviceListPage: PaginationConfig{
			DefaultSize: 100,
			MaxSize:     1000,
		},
		Metrics: MetricsConfig{
			MinValidTimestampMs: defaultMetricsMinValidTimestampMs,
			DefaultRangeLabel:   defaultMetricsRangeLabel,
		},
	}
}

func DefaultAPIConfig() APIConfig {
	return APIConfig{
		TCPAddr:               defaultAPITCPAddr,
		ReadTimeout:           60 * time.Second,
		RegisterAckGraceDelay: 100 * time.Millisecond,
	}
}

func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		RootURL:                     defaultAuthRootURL,
		CookieSecure:                false,
		SessionCookieMaxAgeSeconds:  0,
		RememberCookieMaxAgeSeconds: 86400 * 30,
	}
}

func DefaultCaptchaConfig() CaptchaConfig {
	return CaptchaConfig{
		Provider:      "",
		SiteKey:       "",
		SecretKey:     "",
		VerifyTimeout: 5 * time.Second,
	}
}

func DefaultDeviceManagerConfig() DeviceManagerConfig {
	return DeviceManagerConfig{
		QueueCapacity:     100,
		HeartbeatDeadline: 60 * time.Second,
		ExternalListPage: PaginationConfig{
			DefaultSize: 100,
			MaxSize:     1000,
		},
		ExternalObservationLimit: LimitConfig{
			Default: 1000,
			Max:     10000,
		},
	}
}

func DefaultLoggerConfig() logger.Config {
	return logger.Config{
		Level:     "info",
		Format:    "text",
		AddSource: false,
		Service:   "goster-iot",
		Env:       "dev",
	}
}

func DefaultAppConfig() AppConfig {
	return AppConfig{
		DB:            DefaultDBConfig(),
		Web:           DefaultWebConfig(),
		API:           DefaultAPIConfig(),
		Auth:          DefaultAuthConfig(),
		Captcha:       DefaultCaptchaConfig(),
		DeviceManager: DefaultDeviceManagerConfig(),
		Logger:        DefaultLoggerConfig(),
	}
}

func NormalizeWebConfig(cfg WebConfig) WebConfig {
	base := DefaultWebConfig()
	out := cfg

	out.HTTPAddr = normalizeOrDefault(out.HTTPAddr, base.HTTPAddr)
	out.APICORSAllowOrigins = normalizeOrDefault(out.APICORSAllowOrigins, base.APICORSAllowOrigins)
	out.MaxAPIBodyBytes = normalizePositiveInt64(out.MaxAPIBodyBytes, base.MaxAPIBodyBytes)

	out.DeviceListPage.DefaultSize = normalizePositiveInt(out.DeviceListPage.DefaultSize, base.DeviceListPage.DefaultSize)
	out.DeviceListPage.MaxSize = normalizePositiveInt(out.DeviceListPage.MaxSize, base.DeviceListPage.MaxSize)
	if out.DeviceListPage.DefaultSize > out.DeviceListPage.MaxSize {
		out.DeviceListPage.DefaultSize = out.DeviceListPage.MaxSize
	}

	out.Metrics.MinValidTimestampMs = normalizePositiveInt64(out.Metrics.MinValidTimestampMs, base.Metrics.MinValidTimestampMs)
	out.Metrics.DefaultRangeLabel = normalizeMetricsRangeLabel(out.Metrics.DefaultRangeLabel)
	return out
}

func NormalizeAPIConfig(cfg APIConfig) APIConfig {
	base := DefaultAPIConfig()
	out := cfg
	out.TCPAddr = normalizeOrDefault(out.TCPAddr, base.TCPAddr)
	if out.ReadTimeout <= 0 {
		out.ReadTimeout = base.ReadTimeout
	}
	if out.RegisterAckGraceDelay <= 0 {
		out.RegisterAckGraceDelay = base.RegisterAckGraceDelay
	}
	return out
}

func NormalizeAuthConfig(cfg AuthConfig) AuthConfig {
	base := DefaultAuthConfig()
	out := cfg
	out.RootURL = normalizeOrDefault(out.RootURL, base.RootURL)
	out.SessionCookieMaxAgeSeconds = normalizeZeroOrPositiveInt(out.SessionCookieMaxAgeSeconds, base.SessionCookieMaxAgeSeconds)
	out.RememberCookieMaxAgeSeconds = normalizePositiveInt(out.RememberCookieMaxAgeSeconds, base.RememberCookieMaxAgeSeconds)
	return out
}

func NormalizeCaptchaConfig(cfg CaptchaConfig) CaptchaConfig {
	base := DefaultCaptchaConfig()
	out := cfg
	if out.VerifyTimeout <= 0 {
		out.VerifyTimeout = base.VerifyTimeout
	}
	return out
}

func NormalizeDeviceManagerConfig(cfg DeviceManagerConfig) DeviceManagerConfig {
	base := DefaultDeviceManagerConfig()
	out := cfg
	out.QueueCapacity = normalizePositiveInt(out.QueueCapacity, base.QueueCapacity)
	if out.HeartbeatDeadline <= 0 {
		out.HeartbeatDeadline = base.HeartbeatDeadline
	}

	out.ExternalListPage.DefaultSize = normalizePositiveInt(out.ExternalListPage.DefaultSize, base.ExternalListPage.DefaultSize)
	out.ExternalListPage.MaxSize = normalizePositiveInt(out.ExternalListPage.MaxSize, base.ExternalListPage.MaxSize)
	if out.ExternalListPage.DefaultSize > out.ExternalListPage.MaxSize {
		out.ExternalListPage.DefaultSize = out.ExternalListPage.MaxSize
	}

	out.ExternalObservationLimit.Default = normalizePositiveInt(out.ExternalObservationLimit.Default, base.ExternalObservationLimit.Default)
	out.ExternalObservationLimit.Max = normalizePositiveInt(out.ExternalObservationLimit.Max, base.ExternalObservationLimit.Max)
	if out.ExternalObservationLimit.Default > out.ExternalObservationLimit.Max {
		out.ExternalObservationLimit.Default = out.ExternalObservationLimit.Max
	}
	return out
}

// ResolveCookieSecure 根据配置值推导 Cookie Secure 标志。
func ResolveCookieSecure(rawCookieSecure, rawEnv, rawRootURL string) bool {
	if raw := strings.TrimSpace(rawCookieSecure); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err == nil {
			return v
		}
	}

	env := strings.ToLower(strings.TrimSpace(rawEnv))
	if env == "prod" || env == "production" {
		return true
	}
	rootURL := strings.ToLower(strings.TrimSpace(rawRootURL))
	return strings.HasPrefix(rootURL, "https://")
}

func Load() (AppConfig, error) {
	v := viper.New()
	if err := prepareViper(v); err != nil {
		return AppConfig{}, err
	}
	return loadFromViper(v), nil
}

func prepareViper(v *viper.Viper) error {
	v.SetDefault("db.path", defaultDBPath)
	v.SetDefault("web.http_addr", defaultWebHTTPAddr)
	v.SetDefault("web.api_cors_allow_origins", defaultAPICORSAllowOrigins)
	v.SetDefault("web.max_api_body_bytes", defaultMaxAPIBodyBytes)
	v.SetDefault("web.device_list.default_page_size", 100)
	v.SetDefault("web.device_list.max_page_size", 1000)
	v.SetDefault("web.metrics.min_valid_timestamp_ms", defaultMetricsMinValidTimestampMs)
	v.SetDefault("web.metrics.default_range_label", defaultMetricsRangeLabel)

	v.SetDefault("api.tcp_addr", defaultAPITCPAddr)
	v.SetDefault("api.read_timeout", "60s")
	v.SetDefault("api.register_ack_grace_delay", "100ms")

	v.SetDefault("auth.root_url", defaultAuthRootURL)
	v.SetDefault("auth.session_cookie_max_age_seconds", 0)
	v.SetDefault("auth.remember_cookie_max_age_seconds", 86400*30)

	v.SetDefault("captcha.verify_timeout", "5s")

	v.SetDefault("device_manager.queue_capacity", 100)
	v.SetDefault("device_manager.heartbeat_deadline", "60s")
	v.SetDefault("device_manager.external_list.default_size", 100)
	v.SetDefault("device_manager.external_list.max_size", 1000)
	v.SetDefault("device_manager.external_observation.default_limit", 1000)
	v.SetDefault("device_manager.external_observation.max_limit", 10000)

	v.SetDefault("logger.level", "info")
	v.SetDefault("logger.format", "text")
	v.SetDefault("logger.add_source", false)
	v.SetDefault("logger.service", "goster-iot")

	binds := map[string]string{
		"db.path":                                           "DB_PATH",
		"web.http_addr":                                     "WEB_HTTP_ADDR",
		"web.api_cors_allow_origins":                        "API_CORS_ALLOW_ORIGINS",
		"web.max_api_body_bytes":                            "WEB_API_MAX_BODY_BYTES",
		"web.device_list.default_page_size":                 "WEB_DEVICE_LIST_DEFAULT_PAGE_SIZE",
		"web.device_list.max_page_size":                     "WEB_DEVICE_LIST_MAX_PAGE_SIZE",
		"web.metrics.min_valid_timestamp_ms":                "WEB_METRICS_MIN_VALID_TIMESTAMP_MS",
		"web.metrics.default_range_label":                   "WEB_METRICS_DEFAULT_RANGE_LABEL",
		"api.tcp_addr":                                      "API_TCP_ADDR",
		"api.read_timeout":                                  "API_READ_TIMEOUT",
		"api.register_ack_grace_delay":                      "API_REGISTER_ACK_GRACE_DELAY",
		"auth.root_url":                                     "AUTHBOSS_ROOT_URL",
		"auth.cookie_secure":                                "AUTH_COOKIE_SECURE",
		"auth.github_client_id":                             "GITHUB_CLIENT_ID",
		"auth.github_client_secret":                         "GITHUB_CLIENT_SECRET",
		"auth.session_cookie_max_age_seconds":               "AUTH_SESSION_COOKIE_MAX_AGE_SECONDS",
		"auth.remember_cookie_max_age_seconds":              "AUTH_REMEMBER_COOKIE_MAX_AGE_SECONDS",
		"captcha.provider":                                  "CAPTCHA_PROVIDER",
		"captcha.site_key":                                  "CF_SITE_KEY",
		"captcha.secret_key":                                "CF_SECRET_KEY",
		"captcha.verify_timeout":                            "CAPTCHA_VERIFY_TIMEOUT",
		"device_manager.queue_capacity":                     "DM_QUEUE_CAPACITY",
		"device_manager.heartbeat_deadline":                 "DM_HEARTBEAT_DEADLINE",
		"device_manager.external_list.default_size":         "DM_EXTERNAL_LIST_DEFAULT_SIZE",
		"device_manager.external_list.max_size":             "DM_EXTERNAL_LIST_MAX_SIZE",
		"device_manager.external_observation.default_limit": "DM_EXTERNAL_OBS_DEFAULT_LIMIT",
		"device_manager.external_observation.max_limit":     "DM_EXTERNAL_OBS_MAX_LIMIT",
		"logger.level":                                      "LOG_LEVEL",
		"logger.format":                                     "LOG_FORMAT",
		"logger.add_source":                                 "LOG_ADD_SOURCE",
		"logger.service":                                    "LOG_SERVICE",
		"logger.env":                                        "LOG_ENV",
		"app.env":                                           "APP_ENV",
	}

	for key, envKey := range binds {
		if err := v.BindEnv(key, envKey); err != nil {
			return fmt.Errorf("bind env failed: key=%s env=%s: %w", key, envKey, err)
		}
	}
	return nil
}

func loadFromViper(v *viper.Viper) AppConfig {
	base := DefaultAppConfig()
	logEnv := strings.TrimSpace(v.GetString("logger.env"))
	if logEnv == "" {
		logEnv = strings.TrimSpace(v.GetString("app.env"))
	}
	if logEnv == "" {
		logEnv = base.Logger.Env
	}

	rootURL := strings.TrimSpace(v.GetString("auth.root_url"))
	appEnv := strings.TrimSpace(v.GetString("app.env"))
	cookieSecureRaw := strings.TrimSpace(v.GetString("auth.cookie_secure"))

	readTimeout := parseDurationOrDefault(v.GetString("api.read_timeout"), base.API.ReadTimeout)
	registerAckGrace := parseDurationOrDefault(v.GetString("api.register_ack_grace_delay"), base.API.RegisterAckGraceDelay)
	captchaVerifyTimeout := parseDurationOrDefault(v.GetString("captcha.verify_timeout"), base.Captcha.VerifyTimeout)
	heartbeatDeadline := parseDurationOrDefault(v.GetString("device_manager.heartbeat_deadline"), base.DeviceManager.HeartbeatDeadline)

	out := AppConfig{
		DB: DBConfig{
			Path: normalizeOrDefault(v.GetString("db.path"), base.DB.Path),
		},
		Web: WebConfig{
			HTTPAddr:            strings.TrimSpace(v.GetString("web.http_addr")),
			APICORSAllowOrigins: strings.TrimSpace(v.GetString("web.api_cors_allow_origins")),
			MaxAPIBodyBytes:     normalizePositiveInt64(v.GetInt64("web.max_api_body_bytes"), base.Web.MaxAPIBodyBytes),
			DeviceListPage: PaginationConfig{
				DefaultSize: normalizePositiveInt(v.GetInt("web.device_list.default_page_size"), base.Web.DeviceListPage.DefaultSize),
				MaxSize:     normalizePositiveInt(v.GetInt("web.device_list.max_page_size"), base.Web.DeviceListPage.MaxSize),
			},
			Metrics: MetricsConfig{
				MinValidTimestampMs: normalizePositiveInt64(v.GetInt64("web.metrics.min_valid_timestamp_ms"), base.Web.Metrics.MinValidTimestampMs),
				DefaultRangeLabel:   normalizeMetricsRangeLabel(v.GetString("web.metrics.default_range_label")),
			},
		},
		API: APIConfig{
			TCPAddr:               strings.TrimSpace(v.GetString("api.tcp_addr")),
			ReadTimeout:           readTimeout,
			RegisterAckGraceDelay: registerAckGrace,
		},
		Auth: AuthConfig{
			RootURL:                     rootURL,
			CookieSecure:                ResolveCookieSecure(cookieSecureRaw, appEnv, rootURL),
			GitHubClientID:              strings.TrimSpace(v.GetString("auth.github_client_id")),
			GitHubClientSecret:          strings.TrimSpace(v.GetString("auth.github_client_secret")),
			SessionCookieMaxAgeSeconds:  normalizeZeroOrPositiveInt(v.GetInt("auth.session_cookie_max_age_seconds"), base.Auth.SessionCookieMaxAgeSeconds),
			RememberCookieMaxAgeSeconds: normalizePositiveInt(v.GetInt("auth.remember_cookie_max_age_seconds"), base.Auth.RememberCookieMaxAgeSeconds),
		},
		Captcha: CaptchaConfig{
			Provider:      strings.TrimSpace(v.GetString("captcha.provider")),
			SiteKey:       strings.TrimSpace(v.GetString("captcha.site_key")),
			SecretKey:     strings.TrimSpace(v.GetString("captcha.secret_key")),
			VerifyTimeout: captchaVerifyTimeout,
		},
		DeviceManager: DeviceManagerConfig{
			QueueCapacity:     normalizePositiveInt(v.GetInt("device_manager.queue_capacity"), base.DeviceManager.QueueCapacity),
			HeartbeatDeadline: heartbeatDeadline,
			ExternalListPage: PaginationConfig{
				DefaultSize: normalizePositiveInt(v.GetInt("device_manager.external_list.default_size"), base.DeviceManager.ExternalListPage.DefaultSize),
				MaxSize:     normalizePositiveInt(v.GetInt("device_manager.external_list.max_size"), base.DeviceManager.ExternalListPage.MaxSize),
			},
			ExternalObservationLimit: LimitConfig{
				Default: normalizePositiveInt(v.GetInt("device_manager.external_observation.default_limit"), base.DeviceManager.ExternalObservationLimit.Default),
				Max:     normalizePositiveInt(v.GetInt("device_manager.external_observation.max_limit"), base.DeviceManager.ExternalObservationLimit.Max),
			},
		},
		Logger: logger.Config{
			Level:     normalizeLogLevel(v.GetString("logger.level")),
			Format:    normalizeLogFormat(v.GetString("logger.format")),
			AddSource: v.GetBool("logger.add_source"),
			Service:   normalizeOrDefault(v.GetString("logger.service"), base.Logger.Service),
			Env:       normalizeOrDefault(logEnv, base.Logger.Env),
		},
	}
	out.Web = NormalizeWebConfig(out.Web)
	out.API = NormalizeAPIConfig(out.API)
	out.Auth = NormalizeAuthConfig(out.Auth)
	out.Captcha = NormalizeCaptchaConfig(out.Captcha)
	out.DeviceManager = NormalizeDeviceManagerConfig(out.DeviceManager)
	return out
}

func normalizeLogLevel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug", "info", "warn", "error":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "info"
	}
}

func normalizeLogFormat(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "json", "text":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "text"
	}
}

func normalizeOrDefault(raw string, fallback string) string {
	out := strings.TrimSpace(raw)
	if out == "" {
		return fallback
	}
	return out
}

func normalizePositiveInt(raw int, fallback int) int {
	if raw <= 0 {
		return fallback
	}
	return raw
}

func normalizeZeroOrPositiveInt(raw int, fallback int) int {
	if raw < 0 {
		return fallback
	}
	return raw
}

func normalizePositiveInt64(raw int64, fallback int64) int64 {
	if raw <= 0 {
		return fallback
	}
	return raw
}

func parseDurationOrDefault(raw string, fallback time.Duration) time.Duration {
	val := strings.TrimSpace(raw)
	if val == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(val)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func normalizeMetricsRangeLabel(raw string) string {
	switch strings.TrimSpace(raw) {
	case "1h", "6h", "24h", "7d", "all":
		return strings.TrimSpace(raw)
	default:
		return defaultMetricsRangeLabel
	}
}
