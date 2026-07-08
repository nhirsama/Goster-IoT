package mqtt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/adapter"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
)

type InboundMessage struct {
	Topic      string
	Payload    []byte
	QoS        byte
	Retained   bool
	Duplicate  bool
	ReceivedAt time.Time
	ClientID   string
	Username   string
	AuthUUID   string
	AuthTenant string
	RemoteAddr string
	LocalAddr  string
}

type MappedMessage struct {
	Event adapter.AdapterEvent
	Token string
}

type Mapper struct {
	cfg config.MQTTConfig
}

func NewMapper(cfg config.MQTTConfig) *Mapper {
	cfg.NormalizeMQTT()
	return &Mapper{cfg: cfg}
}

func (m *Mapper) Map(msg InboundMessage) (MappedMessage, error) {
	receivedAt := msg.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	topic := cleanTopic(msg.Topic)
	if topic == "" {
		return MappedMessage{}, fmt.Errorf("mqtt topic 不能为空")
	}
	raw := append([]byte(nil), msg.Payload...)
	contentType := detectContentType(raw)
	payloadMap, _ := parseObject(raw)

	if rest, ok := topicRest(topic, m.cfg.BaseTopic); ok {
		return m.mapGoster(rest, topic, raw, contentType, payloadMap, msg, receivedAt)
	}
	if rest, ok := topicRest(topic, m.cfg.Zigbee2MQTTBaseTopic); ok {
		return m.mapZigbee2MQTT(rest, topic, raw, contentType, payloadMap, msg, receivedAt)
	}
	return MappedMessage{}, fmt.Errorf("topic %q 不匹配 base_topic=%q 或 zigbee2mqtt_base_topic=%q", topic, m.cfg.BaseTopic, m.cfg.Zigbee2MQTTBaseTopic)
}

func (m *Mapper) mapGoster(rest []string, topic string, raw []byte, contentType string, payload map[string]any, msg InboundMessage, receivedAt time.Time) (MappedMessage, error) {
	if len(rest) < 2 {
		return MappedMessage{}, fmt.Errorf("goster mqtt topic 需要形如 %s/{uuid}/{kind}", cleanTopic(m.cfg.BaseTopic))
	}
	uuid := strings.TrimSpace(rest[0])
	kind := strings.TrimSpace(rest[1])
	if uuid == "" || kind == "" {
		return MappedMessage{}, fmt.Errorf("goster mqtt topic 缺少 uuid 或 kind")
	}
	if v := stringValue(payload, "uuid", "device_uuid", "deviceId", "device_id"); v != "" {
		uuid = v
	}
	observedAt := observedTime(payload, receivedAt)
	token := stringValue(payload, "token", "device_token", "password")

	event := baseEvent(m.cfg.Source, topic, raw, contentType, msg, receivedAt)
	event.ProtocolName = "mqtt"
	event.ProtocolVersion = "3.1.1"
	event.Transport = "mqtt"
	event.UUID = uuid
	event.Identity = adapter.Identity{Type: "uuid", Value: uuid}
	event.Identities = []adapter.Identity{{Type: "uuid", Value: uuid}}
	event.Device = &adapter.DeviceDescriptor{
		UUID:       uuid,
		Name:       firstNonEmpty(stringValue(payload, "name", "device_name"), uuid),
		DeviceType: firstNonEmpty(stringValue(payload, "device_type", "type"), "mqtt_device"),
		Labels:     map[string]string{"adapter_protocol": "mqtt", "mqtt_topic": topic},
	}

	switch normalizeKind(kind) {
	case "heartbeat":
		event.Kind = "heartbeat"
		event.Availability = firstNonEmpty(stringValue(payload, "availability", "state", "status"), "online")
	case "ack":
		event.Kind = "command_ack"
		event.Receipt = commandReceipt(payload, raw, observedAt)
	case "log":
		event.Kind = "log"
		event.Log = &adapter.LogRecord{
			Level:      firstNonEmpty(stringValue(payload, "level"), "info"),
			Message:    firstNonEmpty(stringValue(payload, "message", "msg"), string(raw)),
			Namespace:  firstNonEmpty(stringValue(payload, "namespace"), "mqtt"),
			ObservedAt: observedAt,
			Fields:     payload,
		}
	case "event":
		event.Kind = "event"
	case "state":
		event.Kind = "state"
		event.States = flatStates(payload, observedAt)
	case "telemetry":
		event.Kind = "telemetry"
		event.Metrics = append(metricArray(payload, observedAt), flatMetrics(payload, observedAt)...)
		if len(event.Metrics) == 0 {
			event.States = flatStates(payload, observedAt)
			if len(event.States) > 0 {
				event.Kind = "state"
			}
		}
	default:
		event.Kind = kind
		event.Metrics = append(metricArray(payload, observedAt), flatMetrics(payload, observedAt)...)
		event.States = flatStates(payload, observedAt)
	}
	return MappedMessage{Event: event, Token: token}, nil
}

func (m *Mapper) mapZigbee2MQTT(rest []string, topic string, raw []byte, contentType string, payload map[string]any, msg InboundMessage, receivedAt time.Time) (MappedMessage, error) {
	if len(rest) == 0 || rest[0] == "" {
		return MappedMessage{}, fmt.Errorf("zigbee2mqtt topic 缺少 friendly_name")
	}
	if rest[0] == "bridge" {
		return MappedMessage{}, fmt.Errorf("zigbee2mqtt bridge topic 暂不作为设备遥测处理")
	}
	friendly := rest[0]
	subtopic := ""
	if len(rest) > 1 {
		subtopic = strings.Join(rest[1:], "/")
	}
	observedAt := observedTime(payload, receivedAt)
	uuid := externalUUID("zigbee2mqtt", friendly)

	event := baseEvent("zigbee2mqtt", topic, raw, contentType, msg, receivedAt)
	event.ProtocolName = "zigbee2mqtt"
	event.ProtocolVersion = "mqtt"
	event.Transport = "mqtt"
	event.UUID = uuid
	event.Identity = adapter.Identity{Type: "zigbee2mqtt_friendly_name", Value: friendly}
	event.Identities = []adapter.Identity{
		{Type: "zigbee2mqtt_friendly_name", Value: friendly},
		{Type: "mqtt_topic", Value: topic},
	}
	event.Device = &adapter.DeviceDescriptor{
		UUID:           uuid,
		Name:           friendly,
		DeviceType:     "zigbee_node",
		NetworkAddress: friendly,
		Identities: []adapter.Identity{
			{Type: "zigbee2mqtt_friendly_name", Value: friendly},
		},
		Labels: map[string]string{
			"adapter_protocol": "zigbee2mqtt",
			"mqtt_topic":       topic,
		},
		Attributes: map[string]any{"friendly_name": friendly},
	}

	if subtopic == "availability" {
		event.Kind = "availability"
		event.Availability = firstNonEmpty(stringValue(payload, "state", "status"), strings.TrimSpace(string(raw)))
		return MappedMessage{Event: event}, nil
	}

	event.Kind = "telemetry"
	event.Metrics = flatMetrics(payload, observedAt)
	event.States = flatStates(payload, observedAt)
	if len(event.Metrics) == 0 && len(event.States) > 0 {
		event.Kind = "state"
	}
	if len(event.Metrics) == 0 && len(event.States) == 0 {
		event.Kind = "event"
	}
	return MappedMessage{Event: event}, nil
}

func baseEvent(source, topic string, raw []byte, contentType string, msg InboundMessage, receivedAt time.Time) adapter.AdapterEvent {
	if source == "" {
		source = "mqtt"
	}
	event := adapter.AdapterEvent{
		AdapterName:     "mqtt",
		ProtocolName:    "mqtt",
		ProtocolVersion: "3.1.1",
		Transport:       "mqtt",
		ReceivedAt:      receivedAt,
		OccurredAt:      receivedAt,
		RemoteAddr:      msg.RemoteAddr,
		LocalAddr:       msg.LocalAddr,
		Raw:             raw,
		RawContentType:  contentType,
		Labels: map[string]string{
			"adapter_protocol": "mqtt",
			"mqtt_topic":       topic,
			"mqtt_qos":         strconv.Itoa(int(msg.QoS)),
			"mqtt_retained":    strconv.FormatBool(msg.Retained),
			"mqtt_duplicate":   strconv.FormatBool(msg.Duplicate),
			"source":           source,
		},
		Attributes: map[string]any{
			"mqtt_topic":     topic,
			"mqtt_qos":       float64(msg.QoS),
			"mqtt_retained":  msg.Retained,
			"mqtt_duplicate": msg.Duplicate,
		},
		Frame: adapter.FrameInfo{
			PayloadLen: uint32(len(raw)),
			Headers: map[string]string{
				"topic":    topic,
				"qos":      strconv.Itoa(int(msg.QoS)),
				"retained": strconv.FormatBool(msg.Retained),
			},
		},
	}
	if msg.ClientID != "" {
		event.Labels["mqtt_client_id"] = msg.ClientID
		event.Attributes["mqtt_client_id"] = msg.ClientID
		event.Frame.Headers["client_id"] = msg.ClientID
	}
	if msg.Username != "" {
		event.Labels["mqtt_username"] = msg.Username
		event.Attributes["mqtt_username"] = msg.Username
	}
	if msg.AuthUUID != "" {
		event.Labels["mqtt_auth_uuid"] = msg.AuthUUID
		event.Attributes["mqtt_auth_uuid"] = msg.AuthUUID
	}
	if msg.AuthTenant != "" {
		event.Labels["mqtt_auth_tenant"] = msg.AuthTenant
		event.Attributes["mqtt_auth_tenant"] = msg.AuthTenant
	}
	return event
}

func flatMetrics(payload map[string]any, observedAt time.Time) []adapter.MetricPoint {
	if len(payload) == 0 {
		return nil
	}
	specs := []struct {
		keys   []string
		name   string
		unit   string
		legacy uint32
	}{
		{[]string{"temperature"}, "temperature", "°C", 1},
		{[]string{"humidity"}, "humidity", "%", 2},
		{[]string{"illuminance", "illuminance_lux", "lux"}, "illuminance", "lx", 4},
		{[]string{"battery"}, "battery", "%", 0},
		{[]string{"linkquality", "link_quality", "lqi"}, "linkquality", "lqi", 0},
		{[]string{"voltage"}, "voltage", "V", 0},
		{[]string{"current"}, "current", "A", 0},
		{[]string{"power"}, "power", "W", 0},
		{[]string{"energy"}, "energy", "kWh", 0},
		{[]string{"pressure"}, "pressure", "hPa", 0},
		{[]string{"device_temperature"}, "device_temperature", "°C", 0},
	}
	out := make([]adapter.MetricPoint, 0, len(specs))
	for _, spec := range specs {
		if n, ok := numberValue(payload, spec.keys...); ok {
			out = append(out, adapter.MetricPoint{
				Name:             spec.name,
				Value:            adapter.Value{Number: &n},
				Unit:             spec.unit,
				ObservedAt:       observedAt,
				LegacyMetricType: spec.legacy,
				Tags:             map[string]string{"source_field": spec.keys[0]},
			})
		}
	}
	return out
}

func metricArray(payload map[string]any, observedAt time.Time) []adapter.MetricPoint {
	raw, ok := payload["metrics"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]adapter.MetricPoint, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := stringValue(m, "name", "metric", "field")
		if name == "" {
			continue
		}
		n, ok := numberValue(m, "value", "number")
		if !ok {
			continue
		}
		legacy := uint32(0)
		if v, ok := numberValue(m, "legacy_type", "legacy_metric_type", "type"); ok && v >= 0 && v <= math.MaxUint32 {
			legacy = uint32(v)
		}
		out = append(out, adapter.MetricPoint{
			Name:             name,
			Value:            adapter.Value{Number: &n},
			Unit:             stringValue(m, "unit"),
			ObservedAt:       observedTime(m, observedAt),
			LegacyMetricType: legacy,
			Tags:             stringMapValue(m, "tags"),
		})
	}
	return out
}

func flatStates(payload map[string]any, observedAt time.Time) []adapter.StatePoint {
	if len(payload) == 0 {
		return nil
	}
	keys := []string{"state", "occupancy", "contact", "action", "tamper", "water_leak", "smoke", "presence", "switch", "brightness", "color_temp", "battery_low", "child_lock"}
	out := make([]adapter.StatePoint, 0, len(keys))
	for _, key := range keys {
		v, ok := payload[key]
		if !ok || isEmptyValue(v) {
			continue
		}
		out = append(out, adapter.StatePoint{
			Name:       key,
			Value:      adapterValue(v),
			ObservedAt: observedAt,
			EntityID:   key,
			Tags:       map[string]string{"source_field": key},
		})
	}
	return out
}

func adapterValue(v any) adapter.Value {
	switch x := v.(type) {
	case bool:
		return adapter.Value{Bool: &x}
	case string:
		return adapter.Value{String: &x}
	case float64:
		return adapter.Value{Number: &x}
	case float32:
		n := float64(x)
		return adapter.Value{Number: &n}
	case int:
		n := float64(x)
		return adapter.Value{Number: &n}
	case int64:
		n := float64(x)
		return adapter.Value{Number: &n}
	case json.Number:
		if n, err := x.Float64(); err == nil {
			return adapter.Value{Number: &n}
		}
		s := x.String()
		return adapter.Value{String: &s}
	case map[string]any:
		return adapter.Value{JSON: x}
	default:
		s := fmt.Sprint(v)
		return adapter.Value{String: &s}
	}
}

func parseObject(payload []byte) (map[string]any, error) {
	var obj map[string]any
	dec := json.NewDecoder(strings.NewReader(string(payload)))
	dec.UseNumber()
	if err := dec.Decode(&obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func detectContentType(payload []byte) string {
	trimmed := strings.TrimSpace(string(payload))
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return "application/json"
	}
	return "text/plain"
}

func topicRest(topic, base string) ([]string, bool) {
	topicParts := splitTopic(topic)
	baseParts := splitTopic(base)
	if len(baseParts) == 0 || len(topicParts) < len(baseParts) {
		return nil, false
	}
	for i := range baseParts {
		if topicParts[i] != baseParts[i] {
			return nil, false
		}
	}
	return topicParts[len(baseParts):], true
}

func splitTopic(topic string) []string {
	topic = cleanTopic(topic)
	if topic == "" {
		return nil
	}
	return strings.Split(topic, "/")
}

func cleanTopic(topic string) string {
	return strings.Trim(strings.TrimSpace(topic), "/")
}

func normalizeKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "telemetry", "metrics", "metric":
		return "telemetry"
	case "heartbeat", "availability", "presence":
		return "heartbeat"
	case "event", "events":
		return "event"
	case "state", "status":
		return "state"
	case "log", "logs":
		return "log"
	case "ack", "acks", "command_ack", "receipt", "command_receipt":
		return "ack"
	default:
		return strings.ToLower(strings.TrimSpace(kind))
	}
}

func commandReceipt(payload map[string]any, raw []byte, observedAt time.Time) *adapter.CommandReceipt {
	commandID := int64(0)
	if n, ok := numberValue(payload, "command_id", "id"); ok {
		commandID = int64(n)
	}
	status := firstNonEmpty(stringValue(payload, "status", "result"), "acked")
	if strings.EqualFold(status, "ok") || strings.EqualFold(status, "success") {
		status = "acked"
	}
	protocolCode := uint32(0)
	if n, ok := numberValue(payload, "protocol_command_code", "cmd_id", "command_code"); ok && n >= 0 && n <= math.MaxUint32 {
		protocolCode = uint32(n)
	}
	return &adapter.CommandReceipt{
		CommandID:           commandID,
		CommandUUID:         stringValue(payload, "command_uuid", "uuid"),
		Status:              status,
		ProtocolCommandCode: protocolCode,
		Operation:           stringValue(payload, "operation"),
		ErrorText:           stringValue(payload, "error", "error_text", "message"),
		ObservedAt:          observedAt,
		Raw:                 raw,
	}
}

func stringValue(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		v, ok := payload[key]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case string:
			return strings.TrimSpace(x)
		case json.Number:
			return x.String()
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64)
		case bool:
			return strconv.FormatBool(x)
		default:
			return strings.TrimSpace(fmt.Sprint(x))
		}
	}
	return ""
}

func numberValue(payload map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		v, ok := payload[key]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case json.Number:
			n, err := x.Float64()
			return n, err == nil
		case float64:
			return x, true
		case float32:
			return float64(x), true
		case int:
			return float64(x), true
		case int64:
			return float64(x), true
		case string:
			n, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
			return n, err == nil
		}
	}
	return 0, false
}

func observedTime(payload map[string]any, fallback time.Time) time.Time {
	if fallback.IsZero() {
		fallback = time.Now().UTC()
	}
	for _, key := range []string{"observed_at", "timestamp", "ts", "time"} {
		v, ok := payload[key]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case string:
			if t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(x)); err == nil {
				return t.UTC()
			}
			if n, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil {
				return unixFlexible(n)
			}
		case json.Number:
			if n, err := x.Float64(); err == nil {
				return unixFlexible(n)
			}
		case float64:
			return unixFlexible(x)
		}
	}
	return fallback
}

func unixFlexible(v float64) time.Time {
	if v <= 0 {
		return time.Now().UTC()
	}
	if v < 1e11 {
		return time.Unix(int64(v), int64((v-math.Trunc(v))*1e9)).UTC()
	}
	return time.UnixMilli(int64(v)).UTC()
}

func stringMapValue(payload map[string]any, key string) map[string]string {
	raw, ok := payload[key]
	if !ok {
		return nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = fmt.Sprint(v)
	}
	return out
}

func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	return false
}

func externalUUID(source, entityID string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(source)) + ":" + strings.TrimSpace(entityID)))
	return "ext_" + hex.EncodeToString(sum[:12])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
