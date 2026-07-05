package normalizer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/adapter"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Normalizer interface {
	NormalizeEvent(ctx context.Context, event adapter.AdapterEvent) (*ingressv1.CanonicalDeviceEvent, error)
	NormalizeCommand(ctx context.Context, command *ingressv1.CanonicalCommand) (adapter.AdapterCommand, error)
}

type Canonical struct {
	SourceInstance string
}

func New(sourceInstance string) *Canonical {
	return &Canonical{SourceInstance: strings.TrimSpace(sourceInstance)}
}

func (n *Canonical) NormalizeEvent(_ context.Context, event adapter.AdapterEvent) (*ingressv1.CanonicalDeviceEvent, error) {
	receivedAt := event.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = receivedAt
	}
	if event.EventID == "" {
		event.EventID = deterministicEventID(event, occurredAt)
	}
	if event.AdapterName == "" {
		event.AdapterName = "unknown"
	}

	ctx := &ingressv1.IngressContext{
		RequestId:       event.RequestID,
		TraceId:         event.TraceID,
		CorrelationId:   event.CorrelationID,
		SourceInstance:  n.SourceInstance,
		AdapterId:       event.AdapterName,
		ProtocolName:    event.ProtocolName,
		ProtocolVersion: event.ProtocolVersion,
		Transport:       mapTransport(event.Transport),
		TenantHint:      event.TenantHint,
		TenantId:        event.TenantID,
		ReceivedAt:      timestamppb.New(receivedAt),
		Network: &ingressv1.NetworkContext{
			RemoteAddr: event.RemoteAddr,
			LocalAddr:  event.LocalAddr,
		},
		Frame:  frameContext(event.Frame),
		Labels: cloneStringMap(event.Labels),
	}
	if event.Attributes != nil {
		st, err := structpb.NewStruct(event.Attributes)
		if err != nil {
			return nil, fmt.Errorf("转换事件扩展属性失败: %w", err)
		}
		ctx.Extensions = st
	}

	out := &ingressv1.CanonicalDeviceEvent{
		EventId:         event.EventID,
		EventSource:     event.AdapterName,
		SpecVersion:     "goster.ingress.v1",
		EventType:       mapEventType(event.Kind),
		EventTypeName:   event.Kind,
		Context:         ctx,
		PrimaryIdentity: identity(event.Identity),
		Identities:      identities(event.Identities),
		Device:          device(event.Device),
		OccurredAt:      timestamppb.New(occurredAt),
		ReceivedAt:      timestamppb.New(receivedAt),
		IdempotencyKey:  event.EventID,
		CorrelationId:   event.CorrelationID,
		Metrics:         metrics(event.Metrics),
		States:          states(event.States),
		Logs:            logs(event.Log),
		Availability:    mapAvailability(event.Availability),
		CommandReceipt:  receipt(event.Receipt),
		Raw:             raw(event.RawContentType, event.Raw),
	}
	if out.PrimaryIdentity == nil && event.UUID != "" {
		out.PrimaryIdentity = &ingressv1.DeviceIdentity{Type: "uuid", Value: event.UUID}
	}
	if event.UUID != "" {
		if out.Device == nil {
			out.Device = &ingressv1.DeviceDescriptor{}
		}
		out.Device.Uuid = event.UUID
	}
	if event.Attributes != nil {
		st, err := structpb.NewStruct(event.Attributes)
		if err != nil {
			return nil, err
		}
		out.Extensions = st
	}
	return out, nil
}

func (n *Canonical) NormalizeCommand(_ context.Context, command *ingressv1.CanonicalCommand) (adapter.AdapterCommand, error) {
	if command == nil {
		return adapter.AdapterCommand{}, fmt.Errorf("command 不能为空")
	}
	options, err := structToMap(command.AdapterOptions)
	if err != nil {
		return adapter.AdapterCommand{}, err
	}
	payload, contentType, err := rawPayload(command.Payload)
	if err != nil {
		return adapter.AdapterCommand{}, err
	}
	var timeout time.Duration
	if command.Timeout != nil {
		timeout = command.Timeout.AsDuration()
	}
	return adapter.AdapterCommand{
		CommandID:           command.CommandId,
		CommandUUID:         command.CommandUuid,
		TenantID:            command.TenantId,
		UUID:                command.Uuid,
		Operation:           command.Operation,
		ProtocolCommandCode: command.ProtocolCommandCode,
		Target:              adapter.Identity{Type: command.TargetIdentity.GetType(), Value: command.TargetIdentity.GetValue(), Issuer: command.TargetIdentity.GetIssuer()},
		Payload:             payload,
		PayloadContentType:  contentType,
		Properties:          cloneStringMap(command.Properties),
		Options:             options,
		Timeout:             timeout,
		MaxAttempts:         command.MaxAttempts,
		Labels:              cloneStringMap(command.Labels),
	}, nil
}

func deterministicEventID(event adapter.AdapterEvent, occurredAt time.Time) string {
	h := sha256.New()
	parts := []string{
		event.AdapterName,
		event.ProtocolName,
		event.Kind,
		event.Identity.Type,
		event.Identity.Value,
		event.UUID,
		occurredAt.UTC().Format(time.RFC3339Nano),
		fmt.Sprintf("%d", event.Frame.Sequence),
		fmt.Sprintf("%d", event.Frame.CommandCode),
	}
	keys := make([]string, 0, len(event.Labels))
	for k := range event.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, k+"="+event.Labels[k])
	}
	_, _ = h.Write([]byte(strings.Join(parts, "|")))
	if len(event.Raw) > 0 {
		_, _ = h.Write(event.Raw)
	}
	return "evt_" + hex.EncodeToString(h.Sum(nil)[:16])
}

func mapTransport(value string) ingressv1.Transport {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "stream", "tcp", "serial", "websocket":
		return ingressv1.Transport_TRANSPORT_STREAM
	case "message_bus", "bus", "mqtt", "nats", "amqp":
		return ingressv1.Transport_TRANSPORT_MESSAGE_BUS
	case "request_reply", "http", "grpc", "connectrpc":
		return ingressv1.Transport_TRANSPORT_REQUEST_REPLY
	case "datagram", "udp":
		return ingressv1.Transport_TRANSPORT_DATAGRAM
	case "local":
		return ingressv1.Transport_TRANSPORT_LOCAL
	default:
		return ingressv1.Transport_TRANSPORT_UNSPECIFIED
	}
}

func mapEventType(kind string) ingressv1.EventType {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "device_discovered", "discovered":
		return ingressv1.EventType_EVENT_TYPE_DEVICE_DISCOVERED
	case "device_register", "register", "registration":
		return ingressv1.EventType_EVENT_TYPE_DEVICE_REGISTER
	case "heartbeat":
		return ingressv1.EventType_EVENT_TYPE_HEARTBEAT
	case "telemetry", "metrics", "metric":
		return ingressv1.EventType_EVENT_TYPE_TELEMETRY
	case "state":
		return ingressv1.EventType_EVENT_TYPE_STATE
	case "log":
		return ingressv1.EventType_EVENT_TYPE_LOG
	case "event", "device_event":
		return ingressv1.EventType_EVENT_TYPE_DEVICE_EVENT
	case "error", "device_error":
		return ingressv1.EventType_EVENT_TYPE_DEVICE_ERROR
	case "availability":
		return ingressv1.EventType_EVENT_TYPE_AVAILABILITY
	case "topology":
		return ingressv1.EventType_EVENT_TYPE_TOPOLOGY
	case "command_ack":
		return ingressv1.EventType_EVENT_TYPE_COMMAND_ACK
	case "command_failed":
		return ingressv1.EventType_EVENT_TYPE_COMMAND_FAILED
	case "raw":
		return ingressv1.EventType_EVENT_TYPE_RAW
	default:
		return ingressv1.EventType_EVENT_TYPE_UNSPECIFIED
	}
}

func mapAvailability(value string) ingressv1.DeviceAvailability {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "online":
		return ingressv1.DeviceAvailability_DEVICE_AVAILABILITY_ONLINE
	case "delayed":
		return ingressv1.DeviceAvailability_DEVICE_AVAILABILITY_DELAYED
	case "offline":
		return ingressv1.DeviceAvailability_DEVICE_AVAILABILITY_OFFLINE
	default:
		return ingressv1.DeviceAvailability_DEVICE_AVAILABILITY_UNSPECIFIED
	}
}

func mapCommandStatus(value string) ingressv1.CommandStatus {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "queued":
		return ingressv1.CommandStatus_COMMAND_STATUS_QUEUED
	case "sent":
		return ingressv1.CommandStatus_COMMAND_STATUS_SENT
	case "acked", "ack":
		return ingressv1.CommandStatus_COMMAND_STATUS_ACKED
	case "failed", "fail":
		return ingressv1.CommandStatus_COMMAND_STATUS_FAILED
	case "requeued", "requeue":
		return ingressv1.CommandStatus_COMMAND_STATUS_REQUEUED
	case "expired":
		return ingressv1.CommandStatus_COMMAND_STATUS_EXPIRED
	default:
		return ingressv1.CommandStatus_COMMAND_STATUS_UNSPECIFIED
	}
}

func frameContext(in adapter.FrameInfo) *ingressv1.FrameContext {
	if in.MagicNumber == 0 && in.CommandCode == 0 && in.KeyID == 0 && in.Sequence == 0 && !in.IsAck && !in.IsEncrypted && !in.IsCompressed && in.PayloadLen == 0 && len(in.Headers) == 0 {
		return nil
	}
	return &ingressv1.FrameContext{
		MagicNumber:  in.MagicNumber,
		CommandCode:  in.CommandCode,
		KeyId:        in.KeyID,
		Sequence:     in.Sequence,
		IsAck:        in.IsAck,
		IsEncrypted:  in.IsEncrypted,
		IsCompressed: in.IsCompressed,
		PayloadLen:   in.PayloadLen,
		Headers:      cloneStringMap(in.Headers),
	}
}

func identity(in adapter.Identity) *ingressv1.DeviceIdentity {
	if strings.TrimSpace(in.Type) == "" && strings.TrimSpace(in.Value) == "" {
		return nil
	}
	return &ingressv1.DeviceIdentity{Type: in.Type, Value: in.Value, Issuer: in.Issuer}
}

func identities(items []adapter.Identity) []*ingressv1.DeviceIdentity {
	out := make([]*ingressv1.DeviceIdentity, 0, len(items))
	for _, item := range items {
		if v := identity(item); v != nil {
			out = append(out, v)
		}
	}
	return out
}

func device(in *adapter.DeviceDescriptor) *ingressv1.DeviceDescriptor {
	if in == nil {
		return nil
	}
	attrs, _ := structpb.NewStruct(in.Attributes)
	return &ingressv1.DeviceDescriptor{
		Uuid:            in.UUID,
		Name:            in.Name,
		SerialNumber:    in.SerialNumber,
		MacAddress:      in.MACAddress,
		HardwareVersion: in.HardwareVersion,
		SoftwareVersion: in.SoftwareVersion,
		ConfigVersion:   in.ConfigVersion,
		Manufacturer:    in.Manufacturer,
		Vendor:          in.Vendor,
		Model:           in.Model,
		FirmwareVersion: in.FirmwareVersion,
		DeviceType:      in.DeviceType,
		NetworkAddress:  in.NetworkAddress,
		Identities:      identities(in.Identities),
		Labels:          cloneStringMap(in.Labels),
		Attributes:      attrs,
	}
}

func metrics(items []adapter.MetricPoint) []*ingressv1.MetricPoint {
	out := make([]*ingressv1.MetricPoint, 0, len(items))
	for _, item := range items {
		observedAt := item.ObservedAt
		if observedAt.IsZero() {
			observedAt = time.Now().UTC()
		}
		out = append(out, &ingressv1.MetricPoint{
			Name:             item.Name,
			Value:            value(item.Value),
			Unit:             item.Unit,
			ObservedAt:       timestamppb.New(observedAt),
			LegacyMetricType: item.LegacyMetricType,
			Tags:             cloneStringMap(item.Tags),
		})
	}
	return out
}

func states(items []adapter.StatePoint) []*ingressv1.StatePoint {
	out := make([]*ingressv1.StatePoint, 0, len(items))
	for _, item := range items {
		observedAt := item.ObservedAt
		if observedAt.IsZero() {
			observedAt = time.Now().UTC()
		}
		out = append(out, &ingressv1.StatePoint{
			Name:       item.Name,
			Value:      value(item.Value),
			Unit:       item.Unit,
			ObservedAt: timestamppb.New(observedAt),
			EntityId:   item.EntityID,
			Tags:       cloneStringMap(item.Tags),
		})
	}
	return out
}

func logs(item *adapter.LogRecord) []*ingressv1.LogRecord {
	if item == nil {
		return nil
	}
	observedAt := item.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	fields, _ := structpb.NewStruct(item.Fields)
	return []*ingressv1.LogRecord{{
		Level:      mapLogLevel(item.Level),
		Message:    item.Message,
		Namespace:  item.Namespace,
		ObservedAt: timestamppb.New(observedAt),
		Fields:     fields,
	}}
}

func mapLogLevel(level string) ingressv1.LogLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return ingressv1.LogLevel_LOG_LEVEL_DEBUG
	case "info":
		return ingressv1.LogLevel_LOG_LEVEL_INFO
	case "warn", "warning":
		return ingressv1.LogLevel_LOG_LEVEL_WARN
	case "error":
		return ingressv1.LogLevel_LOG_LEVEL_ERROR
	default:
		return ingressv1.LogLevel_LOG_LEVEL_UNSPECIFIED
	}
}

func receipt(in *adapter.CommandReceipt) *ingressv1.CommandReceipt {
	if in == nil {
		return nil
	}
	observedAt := in.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	return &ingressv1.CommandReceipt{
		CommandId:           in.CommandID,
		CommandUuid:         in.CommandUUID,
		Status:              mapCommandStatus(in.Status),
		ProtocolCommandCode: in.ProtocolCommandCode,
		Operation:           in.Operation,
		ErrorText:           in.ErrorText,
		ObservedAt:          timestamppb.New(observedAt),
		Raw:                 raw("application/octet-stream", in.Raw),
	}
}

func value(in adapter.Value) *ingressv1.Value {
	switch {
	case in.Number != nil:
		return &ingressv1.Value{Kind: &ingressv1.Value_NumberValue{NumberValue: *in.Number}}
	case in.String != nil:
		return &ingressv1.Value{Kind: &ingressv1.Value_StringValue{StringValue: *in.String}}
	case in.Bool != nil:
		return &ingressv1.Value{Kind: &ingressv1.Value_BoolValue{BoolValue: *in.Bool}}
	case in.Bytes != nil:
		return &ingressv1.Value{Kind: &ingressv1.Value_BytesValue{BytesValue: in.Bytes}}
	case in.JSON != nil:
		st, _ := structpb.NewStruct(in.JSON)
		return &ingressv1.Value{Kind: &ingressv1.Value_JsonValue{JsonValue: st}}
	default:
		return nil
	}
}

func raw(contentType string, body []byte) *ingressv1.RawPayload {
	if len(body) == 0 {
		return nil
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	out := &ingressv1.RawPayload{ContentType: contentType, Body: body}
	if strings.Contains(contentType, "json") {
		var m map[string]any
		if err := json.Unmarshal(body, &m); err == nil {
			out.Json, _ = structpb.NewStruct(m)
		}
	}
	if strings.HasPrefix(contentType, "text/") || strings.Contains(contentType, "json") {
		out.Text = string(body)
	}
	return out
}

func rawPayload(in *ingressv1.RawPayload) ([]byte, string, error) {
	if in == nil {
		return nil, "", nil
	}
	if len(in.Body) > 0 {
		return in.Body, in.ContentType, nil
	}
	if in.Text != "" {
		ct := in.ContentType
		if ct == "" {
			ct = "text/plain"
		}
		return []byte(in.Text), ct, nil
	}
	if in.Json != nil {
		b, err := json.Marshal(in.Json.AsMap())
		if err != nil {
			return nil, "", err
		}
		ct := in.ContentType
		if ct == "" {
			ct = "application/json"
		}
		return b, ct, nil
	}
	return nil, in.ContentType, nil
}

func structToMap(in *structpb.Struct) (map[string]any, error) {
	if in == nil {
		return nil, nil
	}
	return in.AsMap(), nil
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func Duration(d time.Duration) *durationpb.Duration {
	if d <= 0 {
		return nil
	}
	return durationpb.New(d)
}
