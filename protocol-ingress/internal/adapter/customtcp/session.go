package customtcp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/adapter"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/protocol/gosterwy"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type session struct {
	adapter       *Adapter
	logger        *slog.Logger
	conn          net.Conn
	uuid          string
	tenantID      string
	identity      adapter.Identity
	authenticated bool
	mu            sync.Mutex
	inflight      *adapter.AdapterCommand
	inflightSeq   uint64
	clientPubKey  []byte
	serverPubKey  []byte
	handshakeTS   int64
}

func newSession(a *Adapter, logger *slog.Logger, conn net.Conn) *session {
	return &session{adapter: a, logger: logger, conn: conn}
}

func (s *session) IsAuthenticated() bool { return s.authenticated }
func (s *session) UUID() string          { return s.uuid }

func (s *session) Authenticate(ctx context.Context, payload []byte, packet *gosterwy.Packet, sessionKey []byte) (byte, []byte, error) {
	if s.adapter.core == nil {
		return 0x01, nil, errors.New("coreclient 未配置")
	}
	token, err := s.authTokenFromPayload(payload, sessionKey)
	if err != nil {
		return 0x01, nil, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return 0x01, nil, errors.New("token 不能为空")
	}
	rpcCtx, cancel := s.rpcContext(ctx)
	defer cancel()
	resp, err := s.adapter.core.AuthenticateDevice(rpcCtx, &ingressv1.AuthenticateDeviceRequest{
		Context: s.adapter.ingressContext(s.conn, packet, s),
		Credentials: []*ingressv1.Credential{{
			Type:  "token",
			Value: token,
		}},
		Identities: []*ingressv1.DeviceIdentity{{Type: "token", Value: token}},
	})
	if err != nil {
		return 0x01, nil, err
	}
	if resp.GetStatus() != ingressv1.AuthStatus_AUTH_STATUS_ACCEPTED {
		return 0x01, nil, fmt.Errorf("设备鉴权未通过: %s", resp.GetReason())
	}
	s.bindDevice(resp.GetUuid(), resp.GetTenantId(), adapter.Identity{Type: "uuid", Value: resp.GetUuid()})
	return 0x00, nil, nil
}

func (s *session) Register(ctx context.Context, payload string, packet *gosterwy.Packet) (byte, []byte, error) {
	if s.adapter.core == nil {
		return registrationStatusByte(ingressv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED), nil, errors.New("coreclient 未配置")
	}
	dev, err := gosterwy.ParseRegistrationPayload(payload)
	if err != nil {
		return registrationStatusByte(ingressv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED), nil, err
	}
	req := &ingressv1.RegisterDeviceRequest{
		Context: s.adapter.ingressContext(s.conn, packet, s),
		Device:  deviceDescriptor(dev),
		Raw: &ingressv1.RawPayload{
			ContentType: "text/plain",
			Body:        []byte(payload),
			Text:        payload,
		},
	}
	rpcCtx, cancel := s.rpcContext(ctx)
	defer cancel()
	resp, err := s.adapter.core.RegisterDevice(rpcCtx, req)
	if err != nil && resp == nil {
		return registrationStatusByte(ingressv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED), nil, err
	}
	status := resp.GetStatus()
	if status == ingressv1.RegistrationStatus_REGISTRATION_STATUS_UNSPECIFIED {
		status = ingressv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED
	}
	if status == ingressv1.RegistrationStatus_REGISTRATION_STATUS_ACCEPTED {
		s.bindDevice(resp.GetUuid(), resp.GetTenantId(), adapter.Identity{Type: "uuid", Value: resp.GetUuid()})
		if resp.GetCredential() != nil {
			return 0x00, []byte(resp.GetCredential().GetValue()), err
		}
		return 0x00, nil, err
	}
	return registrationStatusByte(status), nil, err
}

func (s *session) HandleHeartbeat(ctx context.Context, packet *gosterwy.Packet) error {
	if !s.authenticated {
		return errors.New("unauthorized")
	}
	if s.adapter.core == nil {
		return errors.New("coreclient 未配置")
	}
	rpcCtx, cancel := s.rpcContext(ctx)
	defer cancel()
	_, err := s.adapter.core.ReportHeartbeat(rpcCtx, &ingressv1.ReportHeartbeatRequest{
		Context:         s.adapter.ingressContext(s.conn, packet, s),
		PrimaryIdentity: &ingressv1.DeviceIdentity{Type: s.identity.Type, Value: s.identity.Value, Issuer: s.identity.Issuer},
		Uuid:            s.uuid,
		Availability:    ingressv1.DeviceAvailability_DEVICE_AVAILABILITY_ONLINE,
		ObservedAt:      timestamppb.Now(),
	})
	return err
}

func (s *session) HandleMetrics(ctx context.Context, packet *gosterwy.Packet) error {
	if !s.authenticated {
		return errors.New("unauthorized")
	}
	points, err := gosterwy.ParseMetricsPayload(packet.Payload)
	if err != nil {
		return err
	}
	return s.ingestEvent(ctx, packet, adapter.AdapterEvent{
		Kind:           "telemetry",
		UUID:           s.uuid,
		Identity:       s.identity,
		Metrics:        points,
		Raw:            packet.Payload,
		RawContentType: "application/octet-stream",
	})
}

func (s *session) HandleLog(ctx context.Context, packet *gosterwy.Packet) error {
	if !s.authenticated {
		return errors.New("unauthorized")
	}
	record, err := gosterwy.ParseLogPayload(packet.Payload)
	if err != nil {
		return err
	}
	return s.ingestEvent(ctx, packet, adapter.AdapterEvent{
		Kind:           "log",
		UUID:           s.uuid,
		Identity:       s.identity,
		Log:            &record,
		Raw:            packet.Payload,
		RawContentType: "application/octet-stream",
	})
}

func (s *session) HandleEvent(ctx context.Context, packet *gosterwy.Packet) error {
	if !s.authenticated {
		return errors.New("unauthorized")
	}
	return s.ingestEvent(ctx, packet, adapter.AdapterEvent{
		Kind:           "event",
		UUID:           s.uuid,
		Identity:       s.identity,
		Raw:            packet.Payload,
		RawContentType: "application/octet-stream",
	})
}

func (s *session) HandleError(ctx context.Context, packet *gosterwy.Packet) {
	if !s.authenticated {
		return
	}
	if err := s.ingestEvent(ctx, packet, adapter.AdapterEvent{
		Kind:           "error",
		UUID:           s.uuid,
		Identity:       s.identity,
		Raw:            packet.Payload,
		RawContentType: "application/octet-stream",
		Log: &adapter.LogRecord{
			Level:      "error",
			Message:    string(packet.Payload),
			Namespace:  "device",
			ObservedAt: time.Now().UTC(),
		},
	}); err != nil {
		s.logger.Warn("设备错误上报失败", "uuid", s.uuid, "error", err)
	}
}

func (s *session) HandleDownlinkAck(ctx context.Context, cmd gosterwy.CmdID, packet *gosterwy.Packet) {
	if !s.authenticated {
		return
	}
	s.mu.Lock()
	if s.inflight == nil {
		s.mu.Unlock()
		s.logger.Warn("收到无法匹配的下行确认", "uuid", s.uuid, "cmd_id", cmd)
		return
	}
	inflight := *s.inflight
	inflightSeq := s.inflightSeq
	inflightCmd, _ := resolveDownlinkCmd(inflight)
	if inflightCmd != cmd {
		s.mu.Unlock()
		s.logger.Warn("收到与当前待确认命令不匹配的下行确认", "uuid", s.uuid, "cmd_id", cmd, "inflight_cmd_id", inflightCmd, "command_id", inflight.CommandID)
		return
	}
	if inflightSeq != packet.Sequence {
		s.mu.Unlock()
		s.logger.Warn("收到与当前待确认命令序列号不匹配的下行确认", "uuid", s.uuid, "cmd_id", cmd, "ack_seq", packet.Sequence, "inflight_seq", inflightSeq, "command_id", inflight.CommandID)
		return
	}
	commandID := inflight.CommandID
	commandUUID := inflight.CommandUUID
	s.inflight = nil
	s.inflightSeq = 0
	s.mu.Unlock()

	rpcCtx, cancel := s.rpcContext(ctx)
	defer cancel()
	_, err := s.adapter.core.UpdateCommandStatus(rpcCtx, &ingressv1.UpdateCommandStatusRequest{
		Context:             s.adapter.ingressContext(s.conn, packet, s),
		CommandId:           commandID,
		CommandUuid:         commandUUID,
		Status:              ingressv1.CommandStatus_COMMAND_STATUS_ACKED,
		ProtocolCommandCode: uint32(cmd),
		ObservedAt:          timestamppb.Now(),
		Uuid:                s.uuid,
		Operation:           gosterwy.CommandName(cmd),
	})
	if err != nil {
		s.logger.Warn("下行确认状态回填失败", "uuid", s.uuid, "cmd_id", cmd, "command_id", commandID, "error", err)
	}
}

func (s *session) PopCommand(ctx context.Context) (adapter.AdapterCommand, bool) {
	if !s.authenticated || s.adapter.core == nil || s.adapter.normalizer == nil || s.hasInflight() {
		return adapter.AdapterCommand{}, false
	}
	rpcCtx, cancel := s.rpcContext(ctx)
	defer cancel()
	resp, err := s.adapter.core.PullCommands(rpcCtx, &ingressv1.PullCommandsRequest{
		Context:         s.adapter.ingressContext(s.conn, nil, s),
		Uuid:            s.uuid,
		PrimaryIdentity: &ingressv1.DeviceIdentity{Type: s.identity.Type, Value: s.identity.Value, Issuer: s.identity.Issuer},
		MaxCount:        1,
	})
	if err != nil {
		s.logger.Warn("拉取下行消息失败", "uuid", s.uuid, "error", err)
		return adapter.AdapterCommand{}, false
	}
	if len(resp.GetCommands()) == 0 {
		return adapter.AdapterCommand{}, false
	}
	cmd, err := s.adapter.normalizer.NormalizeCommand(rpcCtx, resp.GetCommands()[0])
	if err != nil {
		s.logger.Warn("归一化下行命令失败", "uuid", s.uuid, "error", err)
		return adapter.AdapterCommand{}, false
	}
	return cmd, true
}

func (s *session) MarkDownlinkSent(ctx context.Context, msg adapter.AdapterCommand, cmd gosterwy.CmdID, seq uint64) {
	if !s.authenticated || msg.CommandID <= 0 {
		return
	}
	msgCopy := msg
	s.mu.Lock()
	s.inflight = &msgCopy
	s.inflightSeq = seq
	s.mu.Unlock()

	rpcCtx, cancel := s.rpcContext(ctx)
	defer cancel()
	_, err := s.adapter.core.UpdateCommandStatus(rpcCtx, &ingressv1.UpdateCommandStatusRequest{
		Context:             s.adapter.ingressContext(s.conn, nil, s),
		CommandId:           msg.CommandID,
		CommandUuid:         msg.CommandUUID,
		Status:              ingressv1.CommandStatus_COMMAND_STATUS_SENT,
		ProtocolCommandCode: uint32(cmd),
		ObservedAt:          timestamppb.Now(),
		Uuid:                firstNonEmpty(msg.UUID, s.uuid),
		TargetIdentity:      commandTargetIdentity(msg),
		Command:             canonicalCommandFromAdapter(msg, cmd),
		Operation:           msg.Operation,
	})
	if err != nil {
		s.logger.Warn("下行发送状态回填失败", "uuid", s.uuid, "cmd_id", cmd, "command_id", msg.CommandID, "error", err)
	}
}

func (s *session) FailDownlink(ctx context.Context, msg adapter.AdapterCommand, err error) {
	if !s.authenticated || msg.CommandID <= 0 {
		return
	}
	s.clearInflightByCommandID(msg.CommandID)
	errorText := ""
	if err != nil {
		errorText = err.Error()
	}
	rpcCtx, cancel := s.rpcContext(ctx)
	defer cancel()
	_, updateErr := s.adapter.core.UpdateCommandStatus(rpcCtx, &ingressv1.UpdateCommandStatusRequest{
		Context:        s.adapter.ingressContext(s.conn, nil, s),
		CommandId:      msg.CommandID,
		CommandUuid:    msg.CommandUUID,
		Status:         ingressv1.CommandStatus_COMMAND_STATUS_FAILED,
		ErrorText:      errorText,
		ObservedAt:     timestamppb.Now(),
		Uuid:           firstNonEmpty(msg.UUID, s.uuid),
		TargetIdentity: commandTargetIdentity(msg),
		Command:        canonicalCommandFromAdapter(msg, 0),
		Operation:      msg.Operation,
	})
	if updateErr != nil {
		s.logger.Warn("下行失败状态回填失败", "uuid", s.uuid, "command_id", msg.CommandID, "error", updateErr)
	}
}

func (s *session) RequeueDownlink(ctx context.Context, msg adapter.AdapterCommand, err error) {
	if !s.authenticated || msg.CommandID <= 0 {
		return
	}
	s.clearInflightByCommandID(msg.CommandID)
	errorText := ""
	if err != nil {
		errorText = err.Error()
	}
	cmdID, _ := resolveDownlinkCmd(msg)
	rpcCtx, cancel := s.rpcContext(ctx)
	defer cancel()
	_, updateErr := s.adapter.core.UpdateCommandStatus(rpcCtx, &ingressv1.UpdateCommandStatusRequest{
		Context:        s.adapter.ingressContext(s.conn, nil, s),
		CommandId:      msg.CommandID,
		CommandUuid:    msg.CommandUUID,
		Status:         ingressv1.CommandStatus_COMMAND_STATUS_REQUEUED,
		ErrorText:      errorText,
		ObservedAt:     timestamppb.Now(),
		Uuid:           firstNonEmpty(msg.UUID, s.uuid),
		TargetIdentity: commandTargetIdentity(msg),
		Command:        canonicalCommandFromAdapter(msg, cmdID),
		Operation:      msg.Operation,
	})
	if updateErr != nil {
		s.logger.Warn("下行回队状态回填失败", "uuid", s.uuid, "command_id", msg.CommandID, "error", updateErr)
	}
}

func (s *session) RequeueInflight(ctx context.Context) {
	msg, ok := s.takeInflight()
	if !ok {
		return
	}
	s.RequeueDownlink(ctx, msg, nil)
}

func (s *session) RecordHandshake(clientPubKey, serverPubKey []byte, timestamp int64) {
	s.clientPubKey = append(s.clientPubKey[:0], clientPubKey...)
	s.serverPubKey = append(s.serverPubKey[:0], serverPubKey...)
	s.handshakeTS = timestamp
}

func (s *session) authTokenFromPayload(payload []byte, sessionKey []byte) (string, error) {
	if len(sessionKey) == 0 {
		return "", errors.New("AuthVerify 缺少会话密钥")
	}
	if len(s.serverPubKey) == 0 || s.handshakeTS == 0 {
		return "", errors.New("AuthVerify 缺少握手上下文")
	}
	if len(payload) <= sha256.Size {
		return "", errors.New("AuthVerify payload 缺少 HMAC")
	}
	token := payload[:len(payload)-sha256.Size]
	gotMAC := payload[len(payload)-sha256.Size:]
	wantMAC := authVerifyMAC(sessionKey, s.clientPubKey, s.serverPubKey, s.handshakeTS, token)
	if !hmac.Equal(gotMAC, wantMAC) {
		return "", errors.New("AuthVerify HMAC 校验失败")
	}
	return string(token), nil
}

func (s *session) rpcContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := s.adapter.cfg.RPCTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func (s *session) hasInflight() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inflight != nil
}

func (s *session) clearInflightByCommandID(commandID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inflight != nil && s.inflight.CommandID == commandID {
		s.inflight = nil
		s.inflightSeq = 0
	}
}

func (s *session) takeInflight() (adapter.AdapterCommand, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inflight == nil {
		return adapter.AdapterCommand{}, false
	}
	msg := *s.inflight
	s.inflight = nil
	s.inflightSeq = 0
	return msg, true
}

func (s *session) ingestEvent(ctx context.Context, packet *gosterwy.Packet, event adapter.AdapterEvent) error {
	if s.adapter.core == nil || s.adapter.normalizer == nil {
		return errors.New("coreclient 或 normalizer 未配置")
	}
	rpcCtx, cancel := s.rpcContext(ctx)
	defer cancel()
	now := time.Now().UTC()
	event.AdapterName = s.adapter.Name()
	event.ProtocolName = "goster-wy"
	event.ProtocolVersion = "1"
	event.Transport = "stream"
	event.RemoteAddr = s.conn.RemoteAddr().String()
	event.LocalAddr = s.conn.LocalAddr().String()
	event.TenantID = s.tenantID
	event.ReceivedAt = now
	if event.OccurredAt.IsZero() {
		event.OccurredAt = now
	}
	event.Frame = frameInfo(packet)
	if event.Labels == nil {
		event.Labels = map[string]string{}
	}
	event.Labels["adapter_protocol"] = "goster-wy"
	canonical, err := s.adapter.normalizer.NormalizeEvent(rpcCtx, event)
	if err != nil {
		return err
	}
	_, err = s.adapter.core.IngestEvents(rpcCtx, &ingressv1.IngestEventsRequest{
		Context:             canonical.Context,
		Events:              []*ingressv1.CanonicalDeviceEvent{canonical},
		AllowPartialSuccess: true,
	})
	return err
}

func handshakeResponsePayload(serverPubKey []byte, timestamp int64) []byte {
	payload := make([]byte, 32+8)
	copy(payload, serverPubKey)
	binary.LittleEndian.PutUint64(payload[32:], uint64(timestamp))
	return payload
}

func authVerifyMAC(sessionKey, clientPubKey, serverPubKey []byte, timestamp int64, token []byte) []byte {
	mac := hmac.New(sha256.New, sessionKey)
	_, _ = mac.Write(clientPubKey)
	_, _ = mac.Write(serverPubKey)
	var tsBuf [8]byte
	binary.LittleEndian.PutUint64(tsBuf[:], uint64(timestamp))
	_, _ = mac.Write(tsBuf[:])
	_, _ = mac.Write(token)
	return mac.Sum(nil)
}

func authVerifyPayload(sessionKey, clientPubKey, serverPubKey []byte, timestamp int64, token []byte) []byte {
	payload := make([]byte, 0, len(token)+sha256.Size)
	payload = append(payload, token...)
	payload = append(payload, authVerifyMAC(sessionKey, clientPubKey, serverPubKey, timestamp, token)...)
	return payload
}

func (s *session) bindDevice(uuid string, tenantID string, identity adapter.Identity) {
	s.uuid = uuid
	s.tenantID = tenantID
	if identity.Value == "" && uuid != "" {
		identity = adapter.Identity{Type: "uuid", Value: uuid}
	}
	s.identity = identity
	s.authenticated = true
	s.logger = s.logger.With("uuid", uuid)
}

func commandTargetIdentity(msg adapter.AdapterCommand) *ingressv1.DeviceIdentity {
	if msg.Target.Type == "" && msg.Target.Value == "" {
		if msg.UUID == "" {
			return nil
		}
		return &ingressv1.DeviceIdentity{Type: "uuid", Value: msg.UUID}
	}
	return &ingressv1.DeviceIdentity{Type: msg.Target.Type, Value: msg.Target.Value, Issuer: msg.Target.Issuer}
}

func canonicalCommandFromAdapter(msg adapter.AdapterCommand, cmd gosterwy.CmdID) *ingressv1.CanonicalCommand {
	protocolCode := msg.ProtocolCommandCode
	if protocolCode == 0 && cmd != 0 {
		protocolCode = uint32(cmd)
	}
	return &ingressv1.CanonicalCommand{
		CommandId:           msg.CommandID,
		CommandUuid:         msg.CommandUUID,
		TenantId:            msg.TenantID,
		Uuid:                msg.UUID,
		TargetIdentity:      commandTargetIdentity(msg),
		Operation:           msg.Operation,
		ProtocolCommandCode: protocolCode,
		Payload:             &ingressv1.RawPayload{ContentType: firstNonEmpty(msg.PayloadContentType, "application/octet-stream"), Body: msg.Payload},
		MaxAttempts:         msg.MaxAttempts,
		Properties:          msg.Properties,
		Labels:              msg.Labels,
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func registrationStatusByte(status ingressv1.RegistrationStatus) byte {
	switch status {
	case ingressv1.RegistrationStatus_REGISTRATION_STATUS_ACCEPTED:
		return 0x00
	case ingressv1.RegistrationStatus_REGISTRATION_STATUS_PENDING:
		return 0x02
	case ingressv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED:
		return 0x01
	default:
		return 0x01
	}
}

func deviceDescriptor(dev adapter.DeviceDescriptor) *ingressv1.DeviceDescriptor {
	ids := make([]*ingressv1.DeviceIdentity, 0, len(dev.Identities))
	for _, id := range dev.Identities {
		ids = append(ids, &ingressv1.DeviceIdentity{Type: id.Type, Value: id.Value, Issuer: id.Issuer})
	}
	return &ingressv1.DeviceDescriptor{
		Uuid:            dev.UUID,
		Name:            dev.Name,
		SerialNumber:    dev.SerialNumber,
		MacAddress:      dev.MACAddress,
		HardwareVersion: dev.HardwareVersion,
		SoftwareVersion: dev.SoftwareVersion,
		ConfigVersion:   dev.ConfigVersion,
		Manufacturer:    dev.Manufacturer,
		Vendor:          dev.Vendor,
		Model:           dev.Model,
		FirmwareVersion: dev.FirmwareVersion,
		DeviceType:      dev.DeviceType,
		NetworkAddress:  dev.NetworkAddress,
		Identities:      ids,
		Labels:          dev.Labels,
	}
}
