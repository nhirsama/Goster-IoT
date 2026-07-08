package mqtt

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	mqttserver "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1"
)

type embeddedClientSession struct {
	ClientID  string
	Username  string
	UUID      string
	TenantID  string
	TokenHint string
	Remote    string
	CreatedAt time.Time
	LastSeen  time.Time
}

type embeddedBrokerHook struct {
	mqttserver.HookBase

	adapter  *Adapter
	msgCh    chan<- InboundMessage
	logger   *slog.Logger
	sessions sync.Map // client id -> embeddedClientSession
}

func (a *Adapter) startEmbeddedBroker(ctx context.Context) error {
	msgCh := make(chan InboundMessage, a.cfg.MessageBuffer)
	broker := mqttserver.New(&mqttserver.Options{InlineClient: true, Logger: a.logger})
	hook := &embeddedBrokerHook{
		adapter: a,
		msgCh:   msgCh,
		logger:  a.logger,
	}
	if err := broker.AddHook(hook, nil); err != nil {
		return fmt.Errorf("mqtt embedded broker hook 初始化失败: %w", err)
	}
	tcp := listeners.NewTCP(listeners.Config{
		ID:      "goster-mqtt",
		Address: a.cfg.ListenAddr,
	})
	if err := broker.AddListener(tcp); err != nil {
		return fmt.Errorf("mqtt embedded broker listener 初始化失败: %w", err)
	}
	if err := broker.Serve(); err != nil {
		return fmt.Errorf("mqtt embedded broker 启动失败: %w", err)
	}
	defer broker.Close()

	if a.cfg.DownlinkEnabled {
		go a.runDownlinkLoop(ctx, embeddedDownlinkPublisher{server: broker})
	}

	a.logger.Info("mqtt embedded broker 已启动", "addr", tcp.Address(), "auth_mode", a.cfg.AuthMode)
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-msgCh:
			if err := a.handleMessage(ctx, msg); err != nil {
				a.logger.Warn("mqtt embedded 消息处理失败", "topic", msg.Topic, "client_id", msg.ClientID, "error", err)
			}
		}
	}
}

type embeddedDownlinkPublisher struct {
	server *mqttserver.Server
}

func (p embeddedDownlinkPublisher) Publish(_ context.Context, topic string, qos byte, retained bool, payload []byte) error {
	if p.server == nil {
		return fmt.Errorf("mqtt embedded broker 未初始化")
	}
	return p.server.Publish(topic, payload, retained, qos)
}

func (h *embeddedBrokerHook) ID() string {
	return "goster-mqtt-embedded"
}

func (h *embeddedBrokerHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqttserver.OnConnectAuthenticate,
		mqttserver.OnACLCheck,
		mqttserver.OnPublish,
		mqttserver.OnDisconnect,
	}, []byte{b})
}

func (h *embeddedBrokerHook) OnConnectAuthenticate(cl *mqttserver.Client, pk packets.Packet) bool {
	clientID := strings.TrimSpace(pk.Connect.ClientIdentifier)
	if clientID == "" && cl != nil {
		clientID = strings.TrimSpace(cl.ID)
	}
	token := strings.TrimSpace(string(pk.Connect.Password))
	username := strings.TrimSpace(string(pk.Connect.Username))
	if clientID == "" || token == "" {
		h.logWarn("mqtt connect 鉴权失败：client_id/password 不能为空", clientID, username, "", "")
		return false
	}

	session, ok := h.authenticateByClientPasswordToken(clientID, username, token, remoteAddr(cl))
	if !ok {
		return false
	}
	h.sessions.Store(clientID, session)
	h.logInfo("mqtt connect 鉴权成功", clientID, username, session.UUID, session.TenantID)
	return true
}

func (h *embeddedBrokerHook) authenticateByClientPasswordToken(clientID, username, token, remote string) (embeddedClientSession, bool) {
	now := time.Now().UTC()
	rpcCtx, cancel := h.adapter.rpcContext(context.Background())
	defer cancel()
	resp, err := h.adapter.core.AuthenticateDevice(rpcCtx, &ingressv1.AuthenticateDeviceRequest{
		Context: &ingressv1.IngressContext{
			SourceInstance:  h.adapter.sourceInstance,
			AdapterId:       h.adapter.Name(),
			ProtocolName:    "mqtt",
			ProtocolVersion: "3.1.1",
			Transport:       ingressv1.Transport_TRANSPORT_MESSAGE_BUS,
			ReceivedAt:      nil,
			Network:         &ingressv1.NetworkContext{RemoteAddr: remote},
			Labels: map[string]string{
				"mqtt_auth_mode": "client_password_token",
				"mqtt_client_id": clientID,
			},
		},
		Credentials: []*ingressv1.Credential{{
			Type:  "token",
			Value: token,
		}},
		Identities: []*ingressv1.DeviceIdentity{
			{Type: "uuid", Value: clientID},
			{Type: "mqtt_client_id", Value: clientID},
		},
	})
	if err != nil {
		h.logWarn("mqtt connect 鉴权 RPC 失败", clientID, username, "", err.Error())
		return embeddedClientSession{}, false
	}
	if resp.GetStatus() != ingressv1.AuthStatus_AUTH_STATUS_ACCEPTED {
		h.logWarn("mqtt connect 鉴权未通过", clientID, username, "", resp.GetReason())
		return embeddedClientSession{}, false
	}
	uuid := strings.TrimSpace(resp.GetUuid())
	if uuid == "" {
		h.logWarn("mqtt connect 鉴权失败：core 未返回 uuid", clientID, username, "", "")
		return embeddedClientSession{}, false
	}
	if uuid != clientID {
		h.logWarn("mqtt connect 鉴权失败：client_id 与 token 归属设备不匹配", clientID, username, uuid, "")
		return embeddedClientSession{}, false
	}
	return embeddedClientSession{
		ClientID:  clientID,
		Username:  username,
		UUID:      uuid,
		TenantID:  strings.TrimSpace(resp.GetTenantId()),
		TokenHint: tokenHint(token),
		Remote:    remote,
		CreatedAt: now,
		LastSeen:  now,
	}, true
}

func (h *embeddedBrokerHook) OnACLCheck(cl *mqttserver.Client, topic string, write bool) bool {
	if cl != nil && cl.Net.Inline {
		return true
	}
	session, ok := h.sessionForClient(cl)
	if !ok {
		h.logWarn("mqtt acl 拒绝：无连接身份", clientID(cl), "", "", topic)
		return false
	}
	allowed := h.allowedTopic(session, topic, write)
	if !allowed {
		action := "subscribe"
		if write {
			action = "publish"
		}
		h.logger.Warn("mqtt acl 拒绝", "client_id", session.ClientID, "uuid", session.UUID, "tenant_id", session.TenantID, "action", action, "topic", topic)
		return false
	}
	session.LastSeen = time.Now().UTC()
	h.sessions.Store(session.ClientID, session)
	return true
}

func (h *embeddedBrokerHook) allowedTopic(session embeddedClientSession, topic string, write bool) bool {
	topic = cleanTopic(topic)
	if rest, ok := topicRest(topic, h.adapter.cfg.BaseTopic); ok {
		if len(rest) < 2 {
			return false
		}
		uuid := strings.TrimSpace(rest[0])
		kind := normalizeKind(rest[1])
		if uuid == "" || uuid != session.UUID {
			return false
		}
		if write {
			switch kind {
			case "telemetry", "heartbeat", "event", "ack", "state", "log":
				return len(rest) == 2
			default:
				return false
			}
		}
		return kind == "downlink" && len(rest) == 2
	}
	if _, ok := topicRest(topic, h.adapter.cfg.Zigbee2MQTTBaseTopic); ok {
		return write
	}
	return false
}

func (h *embeddedBrokerHook) OnPublish(cl *mqttserver.Client, pk packets.Packet) (packets.Packet, error) {
	if cl != nil && cl.Net.Inline {
		return pk, nil
	}
	session, ok := h.sessionForClient(cl)
	if !ok {
		return pk, nil
	}
	in := InboundMessage{
		Topic:      pk.TopicName,
		Payload:    append([]byte(nil), pk.Payload...),
		QoS:        pk.FixedHeader.Qos,
		Retained:   pk.FixedHeader.Retain,
		Duplicate:  pk.FixedHeader.Dup,
		ReceivedAt: time.Now().UTC(),
		ClientID:   session.ClientID,
		Username:   session.Username,
		AuthUUID:   session.UUID,
		AuthTenant: session.TenantID,
		RemoteAddr: session.Remote,
	}
	select {
	case h.msgCh <- in:
	default:
		h.logger.Warn("mqtt embedded 消息缓冲已满，丢弃消息", "client_id", session.ClientID, "topic", pk.TopicName)
	}
	return pk, nil
}

func (h *embeddedBrokerHook) OnDisconnect(cl *mqttserver.Client, _ error, _ bool) {
	id := clientID(cl)
	if id != "" {
		h.sessions.Delete(id)
	}
}

func (h *embeddedBrokerHook) sessionForClient(cl *mqttserver.Client) (embeddedClientSession, bool) {
	id := clientID(cl)
	if id == "" {
		return embeddedClientSession{}, false
	}
	value, ok := h.sessions.Load(id)
	if !ok {
		return embeddedClientSession{}, false
	}
	session, ok := value.(embeddedClientSession)
	return session, ok
}

func clientID(cl *mqttserver.Client) string {
	if cl == nil {
		return ""
	}
	return strings.TrimSpace(cl.ID)
}

func remoteAddr(cl *mqttserver.Client) string {
	if cl == nil {
		return ""
	}
	return strings.TrimSpace(cl.Net.Remote)
}

func tokenHint(token string) string {
	token = strings.TrimSpace(token)
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func (h *embeddedBrokerHook) logInfo(msg, clientID, username, uuid, tenantID string) {
	h.logger.Info(msg, "client_id", clientID, "username", username, "uuid", uuid, "tenant_id", tenantID)
}

func (h *embeddedBrokerHook) logWarn(msg, clientID, username, uuid, reason string) {
	h.logger.Warn(msg, "client_id", clientID, "username", username, "uuid", uuid, "reason", reason)
}
