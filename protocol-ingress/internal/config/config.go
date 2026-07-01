package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Service  ServiceConfig
	Server   ServerConfig
	Core     CoreConfig
	Adapters AdapterConfig
}

type ServiceConfig struct {
	Name       string
	Env        string
	InstanceID string
}

type ServerConfig struct {
	HTTPAddr        string
	ShutdownTimeout time.Duration
}

type CoreConfig struct {
	Endpoint string
	Timeout  time.Duration
	Token    string
}

type AdapterConfig struct {
	CustomTCP CustomTCPConfig
}

type CustomTCPConfig struct {
	Enabled               bool
	ListenAddr            string
	ReadTimeout           time.Duration
	IdleTimeout           time.Duration
	RPCTimeout            time.Duration
	RegisterAckGraceDelay time.Duration
	DownlinkMaxBatch      int
}

// Default 返回本地开发可用的默认配置。生产部署应通过环境变量覆盖。
func Default() Config {
	return Config{
		Service: ServiceConfig{
			Name:       "protocol-ingress",
			Env:        "dev",
			InstanceID: "ingress-local-01",
		},
		Server: ServerConfig{
			HTTPAddr:        "127.0.0.1:8090",
			ShutdownTimeout: 10 * time.Second,
		},
		Core: CoreConfig{
			Endpoint: "http://127.0.0.1:8080",
			Timeout:  5 * time.Second,
		},
		Adapters: AdapterConfig{
			CustomTCP: CustomTCPConfig{
				Enabled:               false,
				ListenAddr:            "127.0.0.1:8081",
				ReadTimeout:           60 * time.Second,
				IdleTimeout:           5 * time.Minute,
				RPCTimeout:            5 * time.Second,
				RegisterAckGraceDelay: 300 * time.Millisecond,
				DownlinkMaxBatch:      1,
			},
		},
	}
}

// Load 只从环境变量读取配置，符合容器和 Kubernetes 部署习惯。
// 所有环境变量都是可选的，非法值会直接返回错误，避免服务带着错误配置启动。
func Load() (Config, error) {
	return LoadFromEnv(os.LookupEnv)
}

// LoadFromEnv 方便测试注入环境变量来源。
func LoadFromEnv(lookup func(string) (string, bool)) (Config, error) {
	cfg := Default()
	if lookup == nil {
		lookup = os.LookupEnv
	}

	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_SERVICE_NAME"); ok {
		cfg.Service.Name = v
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_ENV"); ok {
		cfg.Service.Env = v
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_INSTANCE_ID"); ok {
		cfg.Service.InstanceID = v
	} else if v, ok := lookupString(lookup, "HOSTNAME"); ok {
		cfg.Service.InstanceID = v
	}

	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_HTTP_ADDR"); ok {
		cfg.Server.HTTPAddr = v
	} else if v, ok := lookupString(lookup, "PORT"); ok {
		cfg.Server.HTTPAddr = ":" + strings.TrimPrefix(v, ":")
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_SHUTDOWN_TIMEOUT"); ok {
		d, err := parseDuration("PROTOCOL_INGRESS_SHUTDOWN_TIMEOUT", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Server.ShutdownTimeout = d
	}

	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_CORE_ENDPOINT"); ok {
		cfg.Core.Endpoint = v
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_CORE_TIMEOUT"); ok {
		d, err := parseDuration("PROTOCOL_INGRESS_CORE_TIMEOUT", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Core.Timeout = d
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_CORE_TOKEN"); ok {
		cfg.Core.Token = v
	}

	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_CUSTOM_TCP_ENABLED"); ok {
		b, err := parseBool("PROTOCOL_INGRESS_CUSTOM_TCP_ENABLED", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.CustomTCP.Enabled = b
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_CUSTOM_TCP_ADDR"); ok {
		cfg.Adapters.CustomTCP.ListenAddr = v
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_CUSTOM_TCP_READ_TIMEOUT"); ok {
		d, err := parseDuration("PROTOCOL_INGRESS_CUSTOM_TCP_READ_TIMEOUT", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.CustomTCP.ReadTimeout = d
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_CUSTOM_TCP_IDLE_TIMEOUT"); ok {
		d, err := parseDuration("PROTOCOL_INGRESS_CUSTOM_TCP_IDLE_TIMEOUT", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.CustomTCP.IdleTimeout = d
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_CUSTOM_TCP_RPC_TIMEOUT"); ok {
		d, err := parseDuration("PROTOCOL_INGRESS_CUSTOM_TCP_RPC_TIMEOUT", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.CustomTCP.RPCTimeout = d
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_CUSTOM_TCP_REGISTER_ACK_GRACE_DELAY"); ok {
		d, err := parseDuration("PROTOCOL_INGRESS_CUSTOM_TCP_REGISTER_ACK_GRACE_DELAY", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.CustomTCP.RegisterAckGraceDelay = d
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_CUSTOM_TCP_DOWNLINK_MAX_BATCH"); ok {
		n, err := parsePositiveInt("PROTOCOL_INGRESS_CUSTOM_TCP_DOWNLINK_MAX_BATCH", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.CustomTCP.DownlinkMaxBatch = n
	}

	cfg.Normalize()
	return cfg, cfg.Validate()
}

func (c *Config) Normalize() {
	if c.Service.Name == "" {
		c.Service.Name = "protocol-ingress"
	}
	if c.Service.Env == "" {
		c.Service.Env = "dev"
	}
	if c.Service.InstanceID == "" {
		c.Service.InstanceID = "ingress-local-01"
	}
	if c.Server.HTTPAddr == "" {
		c.Server.HTTPAddr = "127.0.0.1:8090"
	}
	if c.Server.ShutdownTimeout <= 0 {
		c.Server.ShutdownTimeout = 10 * time.Second
	}
	if c.Core.Endpoint == "" {
		c.Core.Endpoint = "http://127.0.0.1:8080"
	}
	if c.Core.Timeout <= 0 {
		c.Core.Timeout = 5 * time.Second
	}
	if c.Adapters.CustomTCP.ListenAddr == "" {
		c.Adapters.CustomTCP.ListenAddr = "127.0.0.1:8081"
	}
	if c.Adapters.CustomTCP.ReadTimeout <= 0 {
		c.Adapters.CustomTCP.ReadTimeout = 60 * time.Second
	}
	if c.Adapters.CustomTCP.IdleTimeout <= 0 {
		c.Adapters.CustomTCP.IdleTimeout = 5 * time.Minute
	}
	if c.Adapters.CustomTCP.RPCTimeout <= 0 {
		c.Adapters.CustomTCP.RPCTimeout = 5 * time.Second
	}
	if c.Adapters.CustomTCP.RegisterAckGraceDelay <= 0 {
		c.Adapters.CustomTCP.RegisterAckGraceDelay = 300 * time.Millisecond
	}
	if c.Adapters.CustomTCP.DownlinkMaxBatch <= 0 {
		c.Adapters.CustomTCP.DownlinkMaxBatch = 1
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Service.Name) == "" {
		return errors.New("PROTOCOL_INGRESS_SERVICE_NAME 不能为空")
	}
	if strings.TrimSpace(c.Service.InstanceID) == "" {
		return errors.New("PROTOCOL_INGRESS_INSTANCE_ID 不能为空")
	}
	if strings.TrimSpace(c.Server.HTTPAddr) == "" {
		return errors.New("PROTOCOL_INGRESS_HTTP_ADDR 不能为空")
	}
	if strings.TrimSpace(c.Core.Endpoint) == "" {
		return errors.New("PROTOCOL_INGRESS_CORE_ENDPOINT 不能为空")
	}
	if c.Core.Timeout <= 0 {
		return errors.New("PROTOCOL_INGRESS_CORE_TIMEOUT 必须大于 0")
	}
	if c.Server.ShutdownTimeout <= 0 {
		return errors.New("PROTOCOL_INGRESS_SHUTDOWN_TIMEOUT 必须大于 0")
	}
	if c.Adapters.CustomTCP.Enabled && strings.TrimSpace(c.Adapters.CustomTCP.ListenAddr) == "" {
		return errors.New("PROTOCOL_INGRESS_CUSTOM_TCP_ADDR 不能为空")
	}
	if c.Adapters.CustomTCP.ReadTimeout <= 0 {
		return errors.New("PROTOCOL_INGRESS_CUSTOM_TCP_READ_TIMEOUT 必须大于 0")
	}
	if c.Adapters.CustomTCP.IdleTimeout <= 0 {
		return errors.New("PROTOCOL_INGRESS_CUSTOM_TCP_IDLE_TIMEOUT 必须大于 0")
	}
	if c.Adapters.CustomTCP.RPCTimeout <= 0 {
		return errors.New("PROTOCOL_INGRESS_CUSTOM_TCP_RPC_TIMEOUT 必须大于 0")
	}
	if c.Adapters.CustomTCP.RegisterAckGraceDelay <= 0 {
		return errors.New("PROTOCOL_INGRESS_CUSTOM_TCP_REGISTER_ACK_GRACE_DELAY 必须大于 0")
	}
	if c.Adapters.CustomTCP.DownlinkMaxBatch <= 0 {
		return errors.New("PROTOCOL_INGRESS_CUSTOM_TCP_DOWNLINK_MAX_BATCH 必须大于 0")
	}
	return nil
}

func lookupString(lookup func(string) (string, bool), key string) (string, bool) {
	v, ok := lookup(key)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(v), true
}

func parseDuration(key, value string) (time.Duration, error) {
	d, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s 必须是合法 duration，例如 5s、300ms: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s 必须大于 0", key)
	}
	return d, nil
}

func parseBool(key, value string) (bool, error) {
	b, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("%s 必须是 boolean: %w", key, err)
	}
	return b, nil
}

func parsePositiveInt(key, value string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s 必须是整数: %w", key, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s 必须大于 0", key)
	}
	return n, nil
}
