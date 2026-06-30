package ingress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"google.golang.org/protobuf/types/known/structpb"
)

type CoreService struct {
	registry         inter.DeviceRegistry
	presence         inter.DevicePresence
	telemetry        inter.TelemetryIngestService
	downlinkCommands inter.DownlinkCommandService
	tenantResolver   interface {
		ResolveDeviceTenant(uuid string) (string, error)
	}
}

var errInvalidIngressRequest = errors.New("invalid ingress request")

func NewCoreService(registry inter.DeviceRegistry, presence inter.DevicePresence, telemetry inter.TelemetryIngestService, downlinkCommands inter.DownlinkCommandService, tenantResolver interface {
	ResolveDeviceTenant(uuid string) (string, error)
}) *CoreService {
	return &CoreService{registry: registry, presence: presence, telemetry: telemetry, downlinkCommands: downlinkCommands, tenantResolver: tenantResolver}
}

func (s *CoreService) AuthenticateDevice(ctx context.Context, req *connect.Request[ingressv1.AuthenticateDeviceRequest]) (*connect.Response[ingressv1.AuthenticateDeviceResponse], error) {
	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%w: request is required", errInvalidIngressRequest))
	}
	token := credentialValue(req.Msg.GetCredentials(), "token")
	if token == "" {
		token = identityValue(req.Msg.GetIdentities(), "token")
	}
	if token == "" {
		return connect.NewResponse(&ingressv1.AuthenticateDeviceResponse{Status: ingressv1.AuthStatus_AUTH_STATUS_REJECTED, Reason: "token is required"}), nil
	}
	uuid, err := s.registry.Authenticate(token)
	if err != nil {
		return connect.NewResponse(&ingressv1.AuthenticateDeviceResponse{Status: authStatusFromError(err), Reason: err.Error()}), nil
	}
	tenantID := s.resolveTenant(uuid)
	meta, err := s.registry.GetDeviceMetadata(uuid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load device metadata failed: %w", err))
	}
	return connect.NewResponse(&ingressv1.AuthenticateDeviceResponse{
		Status:   ingressv1.AuthStatus_AUTH_STATUS_ACCEPTED,
		Uuid:     uuid,
		TenantId: tenantID,
		Device:   deviceDescriptor(uuid, meta, tenantID),
	}), nil
}

func (s *CoreService) RegisterDevice(ctx context.Context, req *connect.Request[ingressv1.RegisterDeviceRequest]) (*connect.Response[ingressv1.RegisterDeviceResponse], error) {
	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%w: request is required", errInvalidIngressRequest))
	}
	meta := metadataFromDescriptor(req.Msg.GetDevice())
	if strings.TrimSpace(meta.SerialNumber) == "" && strings.TrimSpace(meta.MACAddress) == "" {
		return connect.NewResponse(&ingressv1.RegisterDeviceResponse{Status: ingressv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED, Reason: "serial_number or mac_address is required"}), nil
	}
	uuid := s.registry.GenerateUUID(meta)
	existing, err := s.registry.GetDeviceMetadata(uuid)
	if err != nil {
		if !errors.Is(err, inter.ErrDeviceNotFound) {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load device metadata failed: %w", err))
		}
		if registerErr := s.registry.RegisterDevice(meta); registerErr != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("init device failed: %w", registerErr))
		}
		return connect.NewResponse(&ingressv1.RegisterDeviceResponse{Status: ingressv1.RegistrationStatus_REGISTRATION_STATUS_PENDING, Uuid: uuid, TenantId: s.resolveTenant(uuid), Device: deviceDescriptor(uuid, meta, s.resolveTenant(uuid))}), nil
	}

	tenantID := s.resolveTenant(uuid)
	switch existing.AuthenticateStatus {
	case inter.AuthenticatePending:
		return connect.NewResponse(&ingressv1.RegisterDeviceResponse{Status: ingressv1.RegistrationStatus_REGISTRATION_STATUS_PENDING, Uuid: uuid, TenantId: tenantID, Device: deviceDescriptor(uuid, existing, tenantID)}), nil
	case inter.AuthenticateRefuse, inter.AuthenticateRevoked:
		return connect.NewResponse(&ingressv1.RegisterDeviceResponse{Status: ingressv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED, Uuid: uuid, TenantId: tenantID, Reason: "device registration refused", Device: deviceDescriptor(uuid, existing, tenantID)}), nil
	case inter.Authenticated:
		return connect.NewResponse(&ingressv1.RegisterDeviceResponse{Status: ingressv1.RegistrationStatus_REGISTRATION_STATUS_ACCEPTED, Uuid: uuid, TenantId: tenantID, Credential: &ingressv1.Credential{Type: "token", Value: existing.Token}, Device: deviceDescriptor(uuid, existing, tenantID)}), nil
	default:
		return connect.NewResponse(&ingressv1.RegisterDeviceResponse{Status: ingressv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED, Uuid: uuid, TenantId: tenantID, Reason: "unknown auth status", Device: deviceDescriptor(uuid, existing, tenantID)}), nil
	}
}

func (s *CoreService) ReportHeartbeat(ctx context.Context, req *connect.Request[ingressv1.ReportHeartbeatRequest]) (*connect.Response[ingressv1.ReportHeartbeatResponse], error) {
	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%w: request is required", errInvalidIngressRequest))
	}
	uuid := strings.TrimSpace(req.Msg.GetUuid())
	if uuid == "" {
		uuid = req.Msg.GetPrimaryIdentity().GetValue()
	}
	if uuid == "" {
		uuid = identityValue(req.Msg.GetIdentities(), "uuid")
	}
	if uuid == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("uuid is required"))
	}
	s.presence.HandleHeartbeat(uuid)
	return connect.NewResponse(&ingressv1.ReportHeartbeatResponse{Uuid: uuid, TenantId: s.resolveTenant(uuid), Availability: ingressv1.DeviceAvailability_DEVICE_AVAILABILITY_ONLINE}), nil
}

func (s *CoreService) IngestEvents(ctx context.Context, req *connect.Request[ingressv1.IngestEventsRequest]) (*connect.Response[ingressv1.IngestEventsResponse], error) {
	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%w: request is required", errInvalidIngressRequest))
	}
	results := make([]*ingressv1.EventIngestResult, 0, len(req.Msg.GetEvents()))
	for _, event := range req.Msg.GetEvents() {
		if event == nil {
			results = append(results, &ingressv1.EventIngestResult{Success: false, ErrorCode: "event_required", ErrorMessage: "event is required"})
			if !req.Msg.GetAllowPartialSuccess() {
				return connect.NewResponse(&ingressv1.IngestEventsResponse{Results: results}), nil
			}
			continue
		}
		uuid := event.GetDevice().GetUuid()
		if uuid == "" {
			uuid = event.GetPrimaryIdentity().GetValue()
		}
		if uuid == "" {
			uuid = identityValue(event.GetIdentities(), "uuid")
		}
		if uuid == "" {
			results = append(results, &ingressv1.EventIngestResult{EventId: event.GetEventId(), Success: false, ErrorCode: "uuid_required", ErrorMessage: "uuid is required"})
			if !req.Msg.GetAllowPartialSuccess() {
				return connect.NewResponse(&ingressv1.IngestEventsResponse{Results: results}), nil
			}
			continue
		}
		if err := s.ingestOne(uuid, event); err != nil {
			results = append(results, &ingressv1.EventIngestResult{EventId: event.GetEventId(), Success: false, Uuid: uuid, TenantId: s.resolveTenant(uuid), ErrorCode: "ingest_failed", ErrorMessage: err.Error()})
			if !req.Msg.GetAllowPartialSuccess() {
				return connect.NewResponse(&ingressv1.IngestEventsResponse{Results: results}), nil
			}
			continue
		}
		results = append(results, &ingressv1.EventIngestResult{EventId: event.GetEventId(), Success: true, Uuid: uuid, TenantId: s.resolveTenant(uuid)})
	}
	return connect.NewResponse(&ingressv1.IngestEventsResponse{Results: results}), nil
}

func (s *CoreService) PullCommands(ctx context.Context, req *connect.Request[ingressv1.PullCommandsRequest]) (*connect.Response[ingressv1.PullCommandsResponse], error) {
	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%w: request is required", errInvalidIngressRequest))
	}
	uuid := strings.TrimSpace(req.Msg.GetUuid())
	if uuid == "" {
		uuid = req.Msg.GetPrimaryIdentity().GetValue()
	}
	if uuid == "" {
		uuid = identityValue(req.Msg.GetIdentities(), "uuid")
	}
	if uuid == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("uuid is required"))
	}
	maxCount := req.Msg.GetMaxCount()
	if maxCount <= 0 {
		maxCount = 1
	}
	commands := make([]*ingressv1.CanonicalCommand, 0, maxCount)
	for i := int32(0); i < maxCount; i++ {
		msg, ok, err := s.downlinkCommands.PopDownlink(uuid)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if !ok {
			break
		}
		commands = append(commands, canonicalCommand(uuid, s.resolveTenant(uuid), msg))
	}
	return connect.NewResponse(&ingressv1.PullCommandsResponse{Commands: commands}), nil
}

func (s *CoreService) UpdateCommandStatus(ctx context.Context, req *connect.Request[ingressv1.UpdateCommandStatusRequest]) (*connect.Response[ingressv1.UpdateCommandStatusResponse], error) {
	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%w: request is required", errInvalidIngressRequest))
	}
	commandID := req.Msg.GetCommandId()
	if commandID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%w: command_id is required", errInvalidIngressRequest))
	}
	var err error
	switch req.Msg.GetStatus() {
	case ingressv1.CommandStatus_COMMAND_STATUS_SENT:
		err = s.downlinkCommands.MarkSent(commandID)
	case ingressv1.CommandStatus_COMMAND_STATUS_ACKED:
		err = s.downlinkCommands.MarkAcked(commandID)
	case ingressv1.CommandStatus_COMMAND_STATUS_FAILED, ingressv1.CommandStatus_COMMAND_STATUS_EXPIRED:
		err = s.downlinkCommands.MarkFailed(commandID, req.Msg.GetErrorText())
	case ingressv1.CommandStatus_COMMAND_STATUS_REQUEUED:
		err = s.requeue(req.Msg)
	default:
		err = fmt.Errorf("%w: unsupported command status: %s", errInvalidIngressRequest, req.Msg.GetStatus())
	}
	if err != nil {
		if errors.Is(err, errInvalidIngressRequest) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingressv1.UpdateCommandStatusResponse{Success: true, Status: req.Msg.GetStatus()}), nil
}

func (s *CoreService) ingestOne(uuid string, event *ingressv1.CanonicalDeviceEvent) error {
	if len(event.GetMetrics()) > 0 {
		if err := s.telemetry.IngestMetrics(uuid, metricPoints(event.GetMetrics())); err != nil {
			return err
		}
	}
	for _, log := range event.GetLogs() {
		if err := s.telemetry.IngestLog(uuid, logUploadData(log)); err != nil {
			return err
		}
	}
	if receipt := event.GetCommandReceipt(); receipt != nil {
		if err := s.updateCommandReceipt(uuid, receipt); err != nil {
			return err
		}
	}
	switch event.GetEventType() {
	case ingressv1.EventType_EVENT_TYPE_DEVICE_ERROR, ingressv1.EventType_EVENT_TYPE_COMMAND_FAILED:
		if payload := rawPayloadBytes(event.GetRaw()); len(payload) > 0 {
			return s.telemetry.IngestDeviceError(uuid, payload)
		}
	case ingressv1.EventType_EVENT_TYPE_DEVICE_EVENT, ingressv1.EventType_EVENT_TYPE_RAW:
		if payload := rawPayloadBytes(event.GetRaw()); len(payload) > 0 {
			return s.telemetry.IngestEvent(uuid, payload)
		}
	}
	return nil
}

func (s *CoreService) requeue(req *ingressv1.UpdateCommandStatusRequest) error {
	if req.GetCommandId() <= 0 {
		return fmt.Errorf("%w: command_id is required for requeue", errInvalidIngressRequest)
	}
	uuid := strings.TrimSpace(req.GetUuid())
	if uuid == "" {
		uuid = strings.TrimSpace(req.GetCommand().GetUuid())
	}
	if uuid == "" {
		uuid = strings.TrimSpace(req.GetTargetIdentity().GetValue())
	}
	if uuid == "" {
		return fmt.Errorf("%w: uuid is required for requeue", errInvalidIngressRequest)
	}
	cmdCode := req.GetProtocolCommandCode()
	if cmdCode == 0 {
		cmdCode = req.GetCommand().GetProtocolCommandCode()
	}
	if cmdCode == 0 {
		return fmt.Errorf("%w: protocol_command_code is required for requeue", errInvalidIngressRequest)
	}
	payload := rawPayloadBytes(req.GetRaw())
	if len(payload) == 0 {
		payload = rawPayloadBytes(req.GetCommand().GetPayload())
	}
	if len(payload) == 0 {
		return fmt.Errorf("%w: raw payload is required for requeue", errInvalidIngressRequest)
	}
	msg := inter.DownlinkMessage{CommandID: req.GetCommandId(), CmdID: inter.CmdID(cmdCode), Payload: payload}
	return s.downlinkCommands.Requeue(uuid, msg)
}

func (s *CoreService) updateCommandReceipt(uuid string, receipt *ingressv1.CommandReceipt) error {
	if receipt.GetCommandId() <= 0 {
		return nil
	}
	switch receipt.GetStatus() {
	case ingressv1.CommandStatus_COMMAND_STATUS_SENT:
		return s.downlinkCommands.MarkSent(receipt.GetCommandId())
	case ingressv1.CommandStatus_COMMAND_STATUS_ACKED:
		return s.downlinkCommands.MarkAcked(receipt.GetCommandId())
	case ingressv1.CommandStatus_COMMAND_STATUS_FAILED, ingressv1.CommandStatus_COMMAND_STATUS_EXPIRED:
		return s.downlinkCommands.MarkFailed(receipt.GetCommandId(), receipt.GetErrorText())
	case ingressv1.CommandStatus_COMMAND_STATUS_REQUEUED:
		cmdCode := receipt.GetProtocolCommandCode()
		payload := rawPayloadBytes(receipt.GetRaw())
		if strings.TrimSpace(uuid) == "" || cmdCode == 0 || len(payload) == 0 {
			return fmt.Errorf("%w: command receipt requeue requires uuid, protocol_command_code and raw payload", errInvalidIngressRequest)
		}
		return s.downlinkCommands.Requeue(uuid, inter.DownlinkMessage{CommandID: receipt.GetCommandId(), CmdID: inter.CmdID(cmdCode), Payload: payload})
	case ingressv1.CommandStatus_COMMAND_STATUS_UNSPECIFIED, ingressv1.CommandStatus_COMMAND_STATUS_QUEUED:
		return nil
	default:
		return fmt.Errorf("%w: unsupported command receipt status: %s", errInvalidIngressRequest, receipt.GetStatus())
	}
}

func (s *CoreService) resolveTenant(uuid string) string {
	if s.tenantResolver != nil && strings.TrimSpace(uuid) != "" {
		if tenantID, err := s.tenantResolver.ResolveDeviceTenant(uuid); err == nil && strings.TrimSpace(tenantID) != "" {
			return strings.TrimSpace(tenantID)
		}
	}
	return inter.DefaultTenantID
}

func credentialValue(items []*ingressv1.Credential, typ string) string {
	for _, item := range items {
		if strings.EqualFold(item.GetType(), typ) {
			return strings.TrimSpace(item.GetValue())
		}
	}
	return ""
}

func identityValue(items []*ingressv1.DeviceIdentity, typ string) string {
	for _, item := range items {
		if strings.EqualFold(item.GetType(), typ) {
			return strings.TrimSpace(item.GetValue())
		}
	}
	return ""
}

func authStatusFromError(err error) ingressv1.AuthStatus {
	switch {
	case errors.Is(err, inter.ErrDevicePending):
		return ingressv1.AuthStatus_AUTH_STATUS_PENDING
	case errors.Is(err, inter.ErrDeviceRefused):
		return ingressv1.AuthStatus_AUTH_STATUS_REJECTED
	case errors.Is(err, inter.ErrDeviceUnknown), errors.Is(err, inter.ErrInvalidToken):
		return ingressv1.AuthStatus_AUTH_STATUS_UNKNOWN
	default:
		return ingressv1.AuthStatus_AUTH_STATUS_REJECTED
	}
}

func metadataFromDescriptor(d *ingressv1.DeviceDescriptor) inter.DeviceMetadata {
	if d == nil {
		return inter.DeviceMetadata{}
	}
	return inter.DeviceMetadata{Name: d.GetName(), SerialNumber: d.GetSerialNumber(), MACAddress: d.GetMacAddress(), HWVersion: d.GetHardwareVersion(), SWVersion: firstNonEmpty(d.GetSoftwareVersion(), d.GetFirmwareVersion()), ConfigVersion: d.GetConfigVersion()}
}

func deviceDescriptor(uuid string, meta inter.DeviceMetadata, tenantID string) *ingressv1.DeviceDescriptor {
	return &ingressv1.DeviceDescriptor{Uuid: uuid, Name: meta.Name, SerialNumber: meta.SerialNumber, MacAddress: meta.MACAddress, HardwareVersion: meta.HWVersion, SoftwareVersion: meta.SWVersion, ConfigVersion: meta.ConfigVersion, Labels: map[string]string{"tenant_id": tenantID}}
}

func metricPoints(items []*ingressv1.MetricPoint) []inter.MetricPoint {
	out := make([]inter.MetricPoint, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		value := float32(item.GetValue().GetNumberValue())
		out = append(out, inter.MetricPoint{Timestamp: timestampMillis(item.GetObservedAt().AsTime()), Value: value, Type: uint8(item.GetLegacyMetricType())})
	}
	return out
}

func logUploadData(log *ingressv1.LogRecord) inter.LogUploadData {
	return inter.LogUploadData{Timestamp: timestampMillis(log.GetObservedAt().AsTime()), Level: logLevel(log.GetLevel()), Message: log.GetMessage()}
}

func rawPayloadBytes(raw *ingressv1.RawPayload) []byte {
	if raw == nil {
		return nil
	}
	if len(raw.GetBody()) > 0 {
		return raw.GetBody()
	}
	if raw.GetText() != "" {
		return []byte(raw.GetText())
	}
	if raw.GetJson() != nil {
		data, err := json.Marshal(raw.GetJson().AsMap())
		if err == nil {
			return data
		}
	}
	return nil
}

func logLevel(level ingressv1.LogLevel) inter.LogLevel {
	switch level {
	case ingressv1.LogLevel_LOG_LEVEL_DEBUG:
		return inter.LogLevelDebug
	case ingressv1.LogLevel_LOG_LEVEL_WARN:
		return inter.LogLevelWarn
	case ingressv1.LogLevel_LOG_LEVEL_ERROR:
		return inter.LogLevelError
	default:
		return inter.LogLevelInfo
	}
}

func canonicalCommand(uuid, tenantID string, msg inter.DownlinkMessage) *ingressv1.CanonicalCommand {
	return &ingressv1.CanonicalCommand{CommandId: msg.CommandID, TenantId: tenantID, Uuid: uuid, TargetIdentity: &ingressv1.DeviceIdentity{Type: "uuid", Value: uuid}, Operation: operationName(msg.CmdID), ProtocolCommandCode: uint32(msg.CmdID), Payload: &ingressv1.RawPayload{ContentType: "application/octet-stream", Body: msg.Payload}, AdapterOptions: structWithCommandCode(msg.CmdID)}
}

func structWithCommandCode(cmd inter.CmdID) *structpb.Struct {
	st, _ := structpb.NewStruct(map[string]interface{}{"command_code": float64(cmd)})
	return st
}

func operationName(cmd inter.CmdID) string {
	switch cmd {
	case inter.CmdConfigPush:
		return "config_push"
	case inter.CmdOtaData:
		return "ota_data"
	case inter.CmdActionExec:
		return "action_exec"
	case inter.CmdScreenWy:
		return "screen_wy"
	default:
		return "unknown"
	}
}

func timestampMillis(t time.Time) int64 {
	if t.IsZero() {
		return time.Now().UnixMilli()
	}
	return t.UnixMilli()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
