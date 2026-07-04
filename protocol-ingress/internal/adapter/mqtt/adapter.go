package mqtt

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/adapter"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/coreclient"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/normalizer"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Adapter struct {
	cfg            config.MQTTConfig
	sourceInstance string
	logger         *slog.Logger
	core           coreclient.Client
	normalizer     normalizer.Normalizer
	mapper         *Mapper
	deviceMu       sync.Mutex
	devices        map[string]deviceSession
}

type Option func(*Adapter)

type deviceSession struct {
	UUID     string
	TenantID string
	Identity adapter.Identity
	LastSeen time.Time
}

func New(cfg config.MQTTConfig, logger *slog.Logger, deps ...Option) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}
	cfg.NormalizeMQTT()
	a := &Adapter{
		cfg:            cfg,
		sourceInstance: "protocol-ingress",
		logger:         logger,
		mapper:         NewMapper(cfg),
		devices:        make(map[string]deviceSession),
	}
	for _, opt := range deps {
		opt(a)
	}
	return a
}

func WithCoreClient(core coreclient.Client) Option {
	return func(a *Adapter) { a.core = core }
}

func WithNormalizer(n normalizer.Normalizer) Option {
	return func(a *Adapter) { a.normalizer = n }
}

func WithSourceInstance(instanceID string) Option {
	return func(a *Adapter) {
		if strings.TrimSpace(instanceID) != "" {
			a.sourceInstance = strings.TrimSpace(instanceID)
		}
	}
}

func WithMapper(m *Mapper) Option {
	return func(a *Adapter) {
		if m != nil {
			a.mapper = m
		}
	}
}

func (a *Adapter) Name() string { return "mqtt" }

func (a *Adapter) Start(ctx context.Context) error {
	if !a.cfg.Enabled {
		a.logger.Info("mqtt adapter 未启用")
		return nil
	}
	if a.core == nil {
		return errors.New("mqtt adapter coreclient 未配置")
	}
	if a.normalizer == nil {
		return errors.New("mqtt adapter normalizer 未配置")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	msgCh := make(chan InboundMessage, a.cfg.MessageBuffer)
	opts := paho.NewClientOptions().
		AddBroker(a.cfg.BrokerURL).
		SetClientID(a.cfg.ClientID).
		SetCleanSession(false).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetKeepAlive(a.cfg.KeepAlive).
		SetConnectTimeout(a.cfg.ConnectTimeout)
	if a.cfg.Username != "" {
		opts.SetUsername(a.cfg.Username)
	}
	if a.cfg.Password != "" {
		opts.SetPassword(a.cfg.Password)
	}
	opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		a.logger.Warn("mqtt broker 连接断开", "error", err)
	})
	opts.SetOnConnectHandler(func(client paho.Client) {
		a.logger.Info("mqtt broker 已连接，开始订阅", "broker", a.cfg.BrokerURL, "client_id", a.cfg.ClientID)
		for _, topic := range a.cfg.SubscribeTopics {
			topic := strings.TrimSpace(topic)
			if topic == "" {
				continue
			}
			token := client.Subscribe(topic, a.cfg.QoS, func(_ paho.Client, message paho.Message) {
				payload := append([]byte(nil), message.Payload()...)
				in := InboundMessage{
					Topic:      message.Topic(),
					Payload:    payload,
					QoS:        message.Qos(),
					Retained:   message.Retained(),
					Duplicate:  message.Duplicate(),
					ReceivedAt: time.Now().UTC(),
				}
				select {
				case msgCh <- in:
				default:
					a.logger.Warn("mqtt adapter 消息缓冲已满，丢弃消息", "topic", message.Topic())
				}
			})
			if !token.WaitTimeout(a.cfg.ConnectTimeout) {
				a.logger.Warn("mqtt 订阅超时", "topic", topic)
				continue
			}
			if err := token.Error(); err != nil {
				a.logger.Warn("mqtt 订阅失败", "topic", topic, "error", err)
				continue
			}
			a.logger.Info("mqtt 订阅成功", "topic", topic, "qos", a.cfg.QoS)
		}
	})

	client := paho.NewClient(opts)
	if token := client.Connect(); !token.WaitTimeout(a.cfg.ConnectTimeout) {
		return fmt.Errorf("mqtt 连接超时: broker=%s", a.cfg.BrokerURL)
	} else if err := token.Error(); err != nil {
		return fmt.Errorf("mqtt 连接失败: %w", err)
	}
	defer client.Disconnect(250)
	if a.cfg.DownlinkEnabled {
		go a.runDownlinkLoop(ctx, client)
	}

	a.logger.Info("mqtt adapter 已启动", "broker", a.cfg.BrokerURL, "client_id", a.cfg.ClientID)
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-msgCh:
			if err := a.handleMessage(ctx, msg); err != nil {
				a.logger.Warn("mqtt 消息处理失败", "topic", msg.Topic, "error", err)
			}
		}
	}
}

func (a *Adapter) handleMessage(ctx context.Context, msg InboundMessage) error {
	mapped, err := a.mapper.Map(msg)
	if err != nil {
		return err
	}
	event := mapped.Event
	if mapped.Token != "" {
		uuid, tenantID, err := a.authenticate(ctx, mapped.Token, event)
		if err != nil {
			return err
		}
		if uuid != "" {
			event.UUID = uuid
			event.Identity = adapter.Identity{Type: "uuid", Value: uuid}
			if event.Device == nil {
				event.Device = &adapter.DeviceDescriptor{}
			}
			event.Device.UUID = uuid
		}
		if tenantID != "" {
			event.TenantID = tenantID
		}
	}
	a.rememberDevice(event)
	switch strings.ToLower(strings.TrimSpace(event.Kind)) {
	case "heartbeat":
		return a.reportHeartbeat(ctx, event)
	case "command_ack", "command_failed":
		return a.updateCommandReceipt(ctx, event)
	default:
		return a.ingestEvent(ctx, event)
	}
}

func (a *Adapter) authenticate(ctx context.Context, token string, event adapter.AdapterEvent) (string, string, error) {
	rpcCtx, cancel := a.rpcContext(ctx)
	defer cancel()
	resp, err := a.core.AuthenticateDevice(rpcCtx, &ingressv1.AuthenticateDeviceRequest{
		Context: a.ingressContext(event),
		Credentials: []*ingressv1.Credential{{
			Type:  "token",
			Value: token,
		}},
		Identities: []*ingressv1.DeviceIdentity{{Type: "token", Value: token}},
	})
	if err != nil {
		return "", "", err
	}
	if resp.GetStatus() != ingressv1.AuthStatus_AUTH_STATUS_ACCEPTED {
		return "", "", fmt.Errorf("mqtt payload token 鉴权未通过: %s", resp.GetReason())
	}
	return resp.GetUuid(), resp.GetTenantId(), nil
}

func (a *Adapter) reportHeartbeat(ctx context.Context, event adapter.AdapterEvent) error {
	if event.UUID == "" && event.Identity.Value == "" {
		return errors.New("mqtt heartbeat 缺少 uuid/identity")
	}
	rpcCtx, cancel := a.rpcContext(ctx)
	defer cancel()
	availability := ingressv1.DeviceAvailability_DEVICE_AVAILABILITY_ONLINE
	if strings.EqualFold(event.Availability, "offline") {
		availability = ingressv1.DeviceAvailability_DEVICE_AVAILABILITY_OFFLINE
	}
	_, err := a.core.ReportHeartbeat(rpcCtx, &ingressv1.ReportHeartbeatRequest{
		Context:         a.ingressContext(event),
		PrimaryIdentity: identity(event.Identity),
		Identities:      identities(event.Identities),
		Uuid:            event.UUID,
		Availability:    availability,
		ObservedAt:      timestamppb.Now(),
	})
	return err
}

func (a *Adapter) ingestEvent(ctx context.Context, event adapter.AdapterEvent) error {
	rpcCtx, cancel := a.rpcContext(ctx)
	defer cancel()
	canonical, err := a.normalizer.NormalizeEvent(rpcCtx, event)
	if err != nil {
		return err
	}
	_, err = a.core.IngestEvents(rpcCtx, &ingressv1.IngestEventsRequest{
		Context:             canonical.Context,
		Events:              []*ingressv1.CanonicalDeviceEvent{canonical},
		AllowPartialSuccess: true,
	})
	return err
}

func (a *Adapter) updateCommandReceipt(ctx context.Context, event adapter.AdapterEvent) error {
	if event.Receipt == nil {
		return errors.New("mqtt ack 缺少 command receipt")
	}
	status := commandStatus(event.Receipt.Status)
	if status == ingressv1.CommandStatus_COMMAND_STATUS_UNSPECIFIED {
		status = ingressv1.CommandStatus_COMMAND_STATUS_ACKED
	}
	rpcCtx, cancel := a.rpcContext(ctx)
	defer cancel()
	_, err := a.core.UpdateCommandStatus(rpcCtx, &ingressv1.UpdateCommandStatusRequest{
		Context:             a.ingressContext(event),
		CommandId:           event.Receipt.CommandID,
		CommandUuid:         event.Receipt.CommandUUID,
		Status:              status,
		ProtocolCommandCode: event.Receipt.ProtocolCommandCode,
		Operation:           event.Receipt.Operation,
		ErrorText:           event.Receipt.ErrorText,
		ObservedAt:          timestamppb.Now(),
		Uuid:                event.UUID,
		TargetIdentity:      identity(event.Identity),
		Raw:                 &ingressv1.RawPayload{ContentType: event.RawContentType, Body: event.Raw},
	})
	return err
}

func (a *Adapter) rememberDevice(event adapter.AdapterEvent) {
	uuid := strings.TrimSpace(event.UUID)
	if uuid == "" {
		return
	}
	identity := event.Identity
	if identity.Value == "" {
		identity = adapter.Identity{Type: "uuid", Value: uuid}
	}
	a.deviceMu.Lock()
	defer a.deviceMu.Unlock()
	a.devices[uuid] = deviceSession{
		UUID:     uuid,
		TenantID: event.TenantID,
		Identity: identity,
		LastSeen: time.Now().UTC(),
	}
}

func (a *Adapter) runDownlinkLoop(ctx context.Context, client paho.Client) {
	interval := a.cfg.DownlinkPollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, dev := range a.deviceSnapshot() {
				if err := a.pullAndPublishDownlinks(ctx, client, dev); err != nil {
					a.logger.Warn("mqtt 下行轮询失败", "uuid", dev.UUID, "error", err)
				}
			}
		}
	}
}

func (a *Adapter) deviceSnapshot() []deviceSession {
	now := time.Now().UTC()
	ttl := a.cfg.DownlinkDeviceTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	a.deviceMu.Lock()
	defer a.deviceMu.Unlock()
	out := make([]deviceSession, 0, len(a.devices))
	for uuid, dev := range a.devices {
		if now.Sub(dev.LastSeen) > ttl {
			delete(a.devices, uuid)
			continue
		}
		out = append(out, dev)
	}
	return out
}

func (a *Adapter) pullAndPublishDownlinks(ctx context.Context, client paho.Client, dev deviceSession) error {
	if dev.UUID == "" {
		return nil
	}
	rpcCtx, cancel := a.rpcContext(ctx)
	defer cancel()
	resp, err := a.core.PullCommands(rpcCtx, &ingressv1.PullCommandsRequest{
		Context:         a.ingressContextForDevice(dev),
		Uuid:            dev.UUID,
		PrimaryIdentity: identity(dev.Identity),
		MaxCount:        int32(a.cfg.DownlinkMaxBatch),
	})
	if err != nil {
		return err
	}
	for _, rawCmd := range resp.GetCommands() {
		cmd, err := a.normalizer.NormalizeCommand(ctx, rawCmd)
		if err != nil {
			a.logger.Warn("mqtt 下行命令归一化失败", "uuid", dev.UUID, "command_id", rawCmd.GetCommandId(), "error", err)
			continue
		}
		if err := a.publishDownlink(ctx, client, dev, cmd); err != nil {
			a.markCommandFailed(ctx, dev, cmd, err)
			continue
		}
		a.markCommandSent(ctx, dev, cmd)
	}
	return nil
}

func (a *Adapter) publishDownlink(ctx context.Context, client paho.Client, dev deviceSession, cmd adapter.AdapterCommand) error {
	topic := renderDownlinkTopic(a.cfg.DownlinkTopic, dev, cmd)
	if topic == "" {
		return errors.New("mqtt 下行 topic 为空")
	}
	token := client.Publish(topic, a.cfg.QoS, a.cfg.DownlinkRetained, cmd.Payload)
	if !token.WaitTimeout(a.cfg.RPCTimeout) {
		return fmt.Errorf("mqtt publish 超时: topic=%s command_id=%d", topic, cmd.CommandID)
	}
	if err := token.Error(); err != nil {
		return err
	}
	a.logger.Info("mqtt 下行命令已发布", "topic", topic, "uuid", dev.UUID, "command_id", cmd.CommandID)
	return nil
}

func (a *Adapter) markCommandSent(ctx context.Context, dev deviceSession, cmd adapter.AdapterCommand) {
	if cmd.CommandID <= 0 {
		return
	}
	rpcCtx, cancel := a.rpcContext(ctx)
	defer cancel()
	if _, err := a.core.UpdateCommandStatus(rpcCtx, &ingressv1.UpdateCommandStatusRequest{
		Context:             a.ingressContextForDevice(dev),
		CommandId:           cmd.CommandID,
		CommandUuid:         cmd.CommandUUID,
		Status:              ingressv1.CommandStatus_COMMAND_STATUS_SENT,
		ProtocolCommandCode: cmd.ProtocolCommandCode,
		ObservedAt:          timestamppb.Now(),
		Uuid:                firstNonEmpty(cmd.UUID, dev.UUID),
		TargetIdentity:      commandTargetIdentity(cmd, dev),
		Operation:           cmd.Operation,
	}); err != nil {
		a.logger.Warn("mqtt 下行 sent 状态回填失败", "uuid", dev.UUID, "command_id", cmd.CommandID, "error", err)
	}
}

func (a *Adapter) markCommandFailed(ctx context.Context, dev deviceSession, cmd adapter.AdapterCommand, cause error) {
	if cmd.CommandID <= 0 {
		return
	}
	errorText := ""
	if cause != nil {
		errorText = cause.Error()
	}
	rpcCtx, cancel := a.rpcContext(ctx)
	defer cancel()
	if _, err := a.core.UpdateCommandStatus(rpcCtx, &ingressv1.UpdateCommandStatusRequest{
		Context:             a.ingressContextForDevice(dev),
		CommandId:           cmd.CommandID,
		CommandUuid:         cmd.CommandUUID,
		Status:              ingressv1.CommandStatus_COMMAND_STATUS_FAILED,
		ProtocolCommandCode: cmd.ProtocolCommandCode,
		ErrorText:           errorText,
		ObservedAt:          timestamppb.Now(),
		Uuid:                firstNonEmpty(cmd.UUID, dev.UUID),
		TargetIdentity:      commandTargetIdentity(cmd, dev),
		Operation:           cmd.Operation,
	}); err != nil {
		a.logger.Warn("mqtt 下行 failed 状态回填失败", "uuid", dev.UUID, "command_id", cmd.CommandID, "error", err)
	}
}

func (a *Adapter) ingressContext(event adapter.AdapterEvent) *ingressv1.IngressContext {
	receivedAt := event.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	return &ingressv1.IngressContext{
		SourceInstance:  a.sourceInstance,
		AdapterId:       a.Name(),
		ProtocolName:    firstNonEmpty(event.ProtocolName, "mqtt"),
		ProtocolVersion: event.ProtocolVersion,
		Transport:       ingressv1.Transport_TRANSPORT_MESSAGE_BUS,
		TenantHint:      event.TenantHint,
		TenantId:        event.TenantID,
		ReceivedAt:      timestamppb.New(receivedAt),
		Network: &ingressv1.NetworkContext{
			RemoteAddr: event.RemoteAddr,
			LocalAddr:  event.LocalAddr,
		},
		Frame: &ingressv1.FrameContext{
			PayloadLen: event.Frame.PayloadLen,
			Headers:    event.Frame.Headers,
		},
		Labels: event.Labels,
	}
}

func (a *Adapter) ingressContextForDevice(dev deviceSession) *ingressv1.IngressContext {
	return &ingressv1.IngressContext{
		SourceInstance:  a.sourceInstance,
		AdapterId:       a.Name(),
		ProtocolName:    "mqtt",
		ProtocolVersion: "3.1.1",
		Transport:       ingressv1.Transport_TRANSPORT_MESSAGE_BUS,
		TenantId:        dev.TenantID,
		ReceivedAt:      timestamppb.Now(),
		Labels: map[string]string{
			"adapter_protocol": "mqtt",
			"source":           a.cfg.Source,
		},
	}
}

func (a *Adapter) rpcContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := a.cfg.RPCTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func commandStatus(status string) ingressv1.CommandStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued":
		return ingressv1.CommandStatus_COMMAND_STATUS_QUEUED
	case "sent":
		return ingressv1.CommandStatus_COMMAND_STATUS_SENT
	case "acked", "ack", "ok", "success", "succeeded":
		return ingressv1.CommandStatus_COMMAND_STATUS_ACKED
	case "failed", "fail", "error":
		return ingressv1.CommandStatus_COMMAND_STATUS_FAILED
	case "expired":
		return ingressv1.CommandStatus_COMMAND_STATUS_EXPIRED
	default:
		return ingressv1.CommandStatus_COMMAND_STATUS_UNSPECIFIED
	}
}

func renderDownlinkTopic(tmpl string, dev deviceSession, cmd adapter.AdapterCommand) string {
	if strings.TrimSpace(tmpl) == "" {
		tmpl = "goster/v1/{tenant}/{uuid}/downlink"
	}
	uuid := firstNonEmpty(cmd.UUID, dev.UUID)
	tenant := firstNonEmpty(cmd.TenantID, dev.TenantID, "tenant_legacy")
	replacements := map[string]string{
		"{uuid}":        uuid,
		"{device_uuid}": uuid,
		"{tenant}":      tenant,
		"{tenant_id}":   tenant,
	}
	out := tmpl
	for key, value := range replacements {
		out = strings.ReplaceAll(out, key, value)
	}
	return strings.Trim(out, "/")
}

func commandTargetIdentity(cmd adapter.AdapterCommand, dev deviceSession) *ingressv1.DeviceIdentity {
	if cmd.Target.Type != "" || cmd.Target.Value != "" {
		return &ingressv1.DeviceIdentity{Type: cmd.Target.Type, Value: cmd.Target.Value, Issuer: cmd.Target.Issuer}
	}
	if dev.Identity.Type != "" || dev.Identity.Value != "" {
		return identity(dev.Identity)
	}
	return &ingressv1.DeviceIdentity{Type: "uuid", Value: firstNonEmpty(cmd.UUID, dev.UUID)}
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
