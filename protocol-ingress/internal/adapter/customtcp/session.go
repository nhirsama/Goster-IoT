package customtcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
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
	inflight      *adapter.AdapterCommand
}

func newSession(a *Adapter, logger *slog.Logger, conn net.Conn) *session {
	return &session{adapter: a, logger: logger, conn: conn}
}

func (s *session) IsAuthenticated() bool { return s.authenticated }
func (s *session) UUID() string          { return s.uuid }

func (s *session) Authenticate(ctx context.Context, token string, packet *gosterwy.Packet) (byte, []byte, error) {
	if s.adapter.core == nil {
		return 0x01, nil, errors.New("coreclient 未配置")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return 0x01, nil, errors.New("token 不能为空")
	}
	resp, err := s.adapter.core.AuthenticateDevice(ctx, &ingressv1.AuthenticateDeviceRequest{
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
	resp, err := s.adapter.core.RegisterDevice(ctx, req)
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
	_, err := s.adapter.core.ReportHeartbeat(ctx, &ingressv1.ReportHeartbeatRequest{
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
	if s.inflight == nil {
		s.logger.Warn("收到无法匹配的下行确认", "uuid", s.uuid, "cmd_id", cmd)
		return
	}
	inflightCmd, _ := resolveDownlinkCmd(*s.inflight)
	if inflightCmd != cmd {
		s.logger.Warn("收到与当前待确认命令不匹配的下行确认", "uuid", s.uuid, "cmd_id", cmd, "inflight_cmd_id", inflightCmd, "command_id", s.inflight.CommandID)
		return
	}
	commandID := s.inflight.CommandID
	commandUUID := s.inflight.CommandUUID
	s.inflight = nil
	_, err := s.adapter.core.UpdateCommandStatus(ctx, &ingressv1.UpdateCommandStatusRequest{
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
	if !s.authenticated || s.inflight != nil || s.adapter.core == nil || s.adapter.normalizer == nil {
		return adapter.AdapterCommand{}, false
	}
	resp, err := s.adapter.core.PullCommands(ctx, &ingressv1.PullCommandsRequest{
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
	cmd, err := s.adapter.normalizer.NormalizeCommand(ctx, resp.GetCommands()[0])
	if err != nil {
		s.logger.Warn("归一化下行命令失败", "uuid", s.uuid, "error", err)
		return adapter.AdapterCommand{}, false
	}
	return cmd, true
}

func (s *session) MarkDownlinkSent(ctx context.Context, msg adapter.AdapterCommand, cmd gosterwy.CmdID) {
	if !s.authenticated || msg.CommandID <= 0 {
		return
	}
	msgCopy := msg
	s.inflight = &msgCopy
	_, err := s.adapter.core.UpdateCommandStatus(ctx, &ingressv1.UpdateCommandStatusRequest{
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
	if s.inflight != nil && s.inflight.CommandID == msg.CommandID {
		s.inflight = nil
	}
	errorText := ""
	if err != nil {
		errorText = err.Error()
	}
	_, updateErr := s.adapter.core.UpdateCommandStatus(ctx, &ingressv1.UpdateCommandStatusRequest{
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
	if s.inflight != nil && s.inflight.CommandID == msg.CommandID {
		s.inflight = nil
	}
	errorText := ""
	if err != nil {
		errorText = err.Error()
	}
	cmdID, _ := resolveDownlinkCmd(msg)
	_, updateErr := s.adapter.core.UpdateCommandStatus(ctx, &ingressv1.UpdateCommandStatusRequest{
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
	if s.inflight == nil {
		return
	}
	s.RequeueDownlink(ctx, *s.inflight, nil)
}

func (s *session) ingestEvent(ctx context.Context, packet *gosterwy.Packet, event adapter.AdapterEvent) error {
	if s.adapter.core == nil || s.adapter.normalizer == nil {
		return errors.New("coreclient 或 normalizer 未配置")
	}
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
	canonical, err := s.adapter.normalizer.NormalizeEvent(ctx, event)
	if err != nil {
		return err
	}
	_, err = s.adapter.core.IngestEvents(ctx, &ingressv1.IngestEventsRequest{
		Context:             canonical.Context,
		Events:              []*ingressv1.CanonicalDeviceEvent{canonical},
		AllowPartialSuccess: true,
	})
	return err
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
