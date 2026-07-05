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
	MQTT      MQTTConfig
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

type MQTTConfig struct {
	Enabled              bool
	BrokerURL            string
	ClientID             string
	Username             string
	Password             string
	SubscribeTopics      []string
	QoS                  byte
	ConnectTimeout       time.Duration
	KeepAlive            time.Duration
	MessageBuffer        int
	RPCTimeout           time.Duration
	BaseTopic            string
	Zigbee2MQTTBaseTopic string
	Source               string
	DownlinkEnabled      bool
	DownlinkTopic        string
	DownlinkPollInterval time.Duration
	DownlinkDeviceTTL    time.Duration
	DownlinkMaxBatch     int
	DownlinkRetained     bool
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
			MQTT: MQTTConfig{
				Enabled:              false,
				BrokerURL:            "tcp://127.0.0.1:1883",
				ClientID:             "protocol-ingress-mqtt",
				SubscribeTopics:      []string{"goster/v1/+/+/telemetry", "goster/v1/+/+/heartbeat", "goster/v1/+/+/event", "goster/v1/+/+/ack"},
				QoS:                  1,
				ConnectTimeout:       5 * time.Second,
				KeepAlive:            30 * time.Second,
				MessageBuffer:        128,
				RPCTimeout:           5 * time.Second,
				BaseTopic:            "goster/v1",
				Zigbee2MQTTBaseTopic: "zigbee2mqtt",
				Source:               "mqtt",
				DownlinkEnabled:      true,
				DownlinkTopic:        "goster/v1/{tenant}/{uuid}/downlink",
				DownlinkPollInterval: 2 * time.Second,
				DownlinkDeviceTTL:    10 * time.Minute,
				DownlinkMaxBatch:     1,
				DownlinkRetained:     false,
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
	} else if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_TOKEN"); ok {
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

	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_ENABLED"); ok {
		b, err := parseBool("PROTOCOL_INGRESS_MQTT_ENABLED", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.MQTT.Enabled = b
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_BROKER_URL"); ok {
		cfg.Adapters.MQTT.BrokerURL = v
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_CLIENT_ID"); ok {
		cfg.Adapters.MQTT.ClientID = v
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_USERNAME"); ok {
		cfg.Adapters.MQTT.Username = v
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_PASSWORD"); ok {
		cfg.Adapters.MQTT.Password = v
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_SUBSCRIBE_TOPICS"); ok {
		cfg.Adapters.MQTT.SubscribeTopics = parseCSV(v)
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_QOS"); ok {
		n, err := parseNonNegativeInt("PROTOCOL_INGRESS_MQTT_QOS", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.MQTT.QoS = byte(n)
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_CONNECT_TIMEOUT"); ok {
		d, err := parseDuration("PROTOCOL_INGRESS_MQTT_CONNECT_TIMEOUT", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.MQTT.ConnectTimeout = d
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_KEEP_ALIVE"); ok {
		d, err := parseDuration("PROTOCOL_INGRESS_MQTT_KEEP_ALIVE", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.MQTT.KeepAlive = d
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_MESSAGE_BUFFER"); ok {
		n, err := parsePositiveInt("PROTOCOL_INGRESS_MQTT_MESSAGE_BUFFER", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.MQTT.MessageBuffer = n
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_RPC_TIMEOUT"); ok {
		d, err := parseDuration("PROTOCOL_INGRESS_MQTT_RPC_TIMEOUT", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.MQTT.RPCTimeout = d
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_BASE_TOPIC"); ok {
		cfg.Adapters.MQTT.BaseTopic = v
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_ZIGBEE2MQTT_BASE_TOPIC"); ok {
		cfg.Adapters.MQTT.Zigbee2MQTTBaseTopic = v
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_SOURCE"); ok {
		cfg.Adapters.MQTT.Source = v
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_DOWNLINK_ENABLED"); ok {
		b, err := parseBool("PROTOCOL_INGRESS_MQTT_DOWNLINK_ENABLED", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.MQTT.DownlinkEnabled = b
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_DOWNLINK_TOPIC"); ok {
		cfg.Adapters.MQTT.DownlinkTopic = v
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_DOWNLINK_POLL_INTERVAL"); ok {
		d, err := parseDuration("PROTOCOL_INGRESS_MQTT_DOWNLINK_POLL_INTERVAL", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.MQTT.DownlinkPollInterval = d
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_DOWNLINK_DEVICE_TTL"); ok {
		d, err := parseDuration("PROTOCOL_INGRESS_MQTT_DOWNLINK_DEVICE_TTL", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.MQTT.DownlinkDeviceTTL = d
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_DOWNLINK_MAX_BATCH"); ok {
		n, err := parsePositiveInt("PROTOCOL_INGRESS_MQTT_DOWNLINK_MAX_BATCH", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.MQTT.DownlinkMaxBatch = n
	}
	if v, ok := lookupString(lookup, "PROTOCOL_INGRESS_MQTT_DOWNLINK_RETAINED"); ok {
		b, err := parseBool("PROTOCOL_INGRESS_MQTT_DOWNLINK_RETAINED", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Adapters.MQTT.DownlinkRetained = b
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
	if c.Adapters.MQTT.BrokerURL == "" {
		c.Adapters.MQTT.BrokerURL = "tcp://127.0.0.1:1883"
	}
	if c.Adapters.MQTT.ClientID == "" {
		c.Adapters.MQTT.ClientID = "protocol-ingress-mqtt"
	}
	if len(c.Adapters.MQTT.SubscribeTopics) == 0 {
		c.Adapters.MQTT.SubscribeTopics = []string{"goster/v1/+/+/telemetry", "goster/v1/+/+/heartbeat", "goster/v1/+/+/event", "goster/v1/+/+/ack"}
	}
	if c.Adapters.MQTT.ConnectTimeout <= 0 {
		c.Adapters.MQTT.ConnectTimeout = 5 * time.Second
	}
	if c.Adapters.MQTT.KeepAlive <= 0 {
		c.Adapters.MQTT.KeepAlive = 30 * time.Second
	}
	if c.Adapters.MQTT.MessageBuffer <= 0 {
		c.Adapters.MQTT.MessageBuffer = 128
	}
	if c.Adapters.MQTT.RPCTimeout <= 0 {
		c.Adapters.MQTT.RPCTimeout = 5 * time.Second
	}
	if c.Adapters.MQTT.BaseTopic == "" {
		c.Adapters.MQTT.BaseTopic = "goster/v1"
	}
	if c.Adapters.MQTT.Zigbee2MQTTBaseTopic == "" {
		c.Adapters.MQTT.Zigbee2MQTTBaseTopic = "zigbee2mqtt"
	}
	if c.Adapters.MQTT.Source == "" {
		c.Adapters.MQTT.Source = "mqtt"
	}
	if c.Adapters.MQTT.DownlinkTopic == "" {
		c.Adapters.MQTT.DownlinkTopic = "goster/v1/{tenant}/{uuid}/downlink"
	}
	if c.Adapters.MQTT.DownlinkPollInterval <= 0 {
		c.Adapters.MQTT.DownlinkPollInterval = 2 * time.Second
	}
	if c.Adapters.MQTT.DownlinkDeviceTTL <= 0 {
		c.Adapters.MQTT.DownlinkDeviceTTL = 10 * time.Minute
	}
	if c.Adapters.MQTT.DownlinkMaxBatch <= 0 {
		c.Adapters.MQTT.DownlinkMaxBatch = 1
	}
}

func (c *MQTTConfig) NormalizeMQTT() {
	if c == nil {
		return
	}
	wrapper := Config{Adapters: AdapterConfig{MQTT: *c}}
	wrapper.Normalize()
	*c = wrapper.Adapters.MQTT
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
	if c.Adapters.MQTT.Enabled && strings.TrimSpace(c.Adapters.MQTT.BrokerURL) == "" {
		return errors.New("PROTOCOL_INGRESS_MQTT_BROKER_URL 不能为空")
	}
	if c.Adapters.MQTT.Enabled && strings.TrimSpace(c.Adapters.MQTT.ClientID) == "" {
		return errors.New("PROTOCOL_INGRESS_MQTT_CLIENT_ID 不能为空")
	}
	if c.Adapters.MQTT.Enabled && len(c.Adapters.MQTT.SubscribeTopics) == 0 {
		return errors.New("PROTOCOL_INGRESS_MQTT_SUBSCRIBE_TOPICS 不能为空")
	}
	if c.Adapters.MQTT.QoS > 2 {
		return errors.New("PROTOCOL_INGRESS_MQTT_QOS 必须是 0、1 或 2")
	}
	if c.Adapters.MQTT.ConnectTimeout <= 0 {
		return errors.New("PROTOCOL_INGRESS_MQTT_CONNECT_TIMEOUT 必须大于 0")
	}
	if c.Adapters.MQTT.KeepAlive <= 0 {
		return errors.New("PROTOCOL_INGRESS_MQTT_KEEP_ALIVE 必须大于 0")
	}
	if c.Adapters.MQTT.MessageBuffer <= 0 {
		return errors.New("PROTOCOL_INGRESS_MQTT_MESSAGE_BUFFER 必须大于 0")
	}
	if c.Adapters.MQTT.RPCTimeout <= 0 {
		return errors.New("PROTOCOL_INGRESS_MQTT_RPC_TIMEOUT 必须大于 0")
	}
	if strings.TrimSpace(c.Adapters.MQTT.BaseTopic) == "" {
		return errors.New("PROTOCOL_INGRESS_MQTT_BASE_TOPIC 不能为空")
	}
	if strings.TrimSpace(c.Adapters.MQTT.Zigbee2MQTTBaseTopic) == "" {
		return errors.New("PROTOCOL_INGRESS_MQTT_ZIGBEE2MQTT_BASE_TOPIC 不能为空")
	}
	if c.Adapters.MQTT.DownlinkEnabled && strings.TrimSpace(c.Adapters.MQTT.DownlinkTopic) == "" {
		return errors.New("PROTOCOL_INGRESS_MQTT_DOWNLINK_TOPIC 不能为空")
	}
	if c.Adapters.MQTT.DownlinkPollInterval <= 0 {
		return errors.New("PROTOCOL_INGRESS_MQTT_DOWNLINK_POLL_INTERVAL 必须大于 0")
	}
	if c.Adapters.MQTT.DownlinkDeviceTTL <= 0 {
		return errors.New("PROTOCOL_INGRESS_MQTT_DOWNLINK_DEVICE_TTL 必须大于 0")
	}
	if c.Adapters.MQTT.DownlinkMaxBatch <= 0 {
		return errors.New("PROTOCOL_INGRESS_MQTT_DOWNLINK_MAX_BATCH 必须大于 0")
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

func parseNonNegativeInt(key, value string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s 必须是整数: %w", key, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("%s 必须大于等于 0", key)
	}
	return n, nil
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
