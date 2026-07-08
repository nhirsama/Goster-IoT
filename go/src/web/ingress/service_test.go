package ingress

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeRegistry struct {
	metas          map[string]inter.DeviceMetadata
	tokens         map[string]string
	authErr        map[string]error
	generatedUUID  string
	registered     []inter.DeviceMetadata
	registerErr    error
	lastAuthToken  string
	metadataErrors map[string]error
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{
		metas:          map[string]inter.DeviceMetadata{},
		tokens:         map[string]string{},
		authErr:        map[string]error{},
		metadataErrors: map[string]error{},
	}
}

func (f *fakeRegistry) GenerateUUID(meta inter.DeviceMetadata) string {
	if f.generatedUUID != "" {
		return f.generatedUUID
	}
	return "uuid-" + strings.TrimSpace(meta.SerialNumber) + strings.TrimSpace(meta.MACAddress)
}

func (f *fakeRegistry) RegisterDevice(meta inter.DeviceMetadata) error {
	if f.registerErr != nil {
		return f.registerErr
	}
	f.registered = append(f.registered, meta)
	uuid := f.GenerateUUID(meta)
	meta.AuthenticateStatus = inter.AuthenticatePending
	f.metas[uuid] = meta
	return nil
}

func (f *fakeRegistry) ProvisionDevice(scope inter.Scope, meta inter.DeviceMetadata) (string, string, error) {
	uuid := f.GenerateUUID(meta)
	token := "token-" + uuid
	meta.AuthenticateStatus = inter.Authenticated
	meta.Token = token
	f.metas[uuid] = meta
	f.tokens[token] = uuid
	return uuid, token, nil
}

func (f *fakeRegistry) Authenticate(token string) (string, error) {
	f.lastAuthToken = token
	if err := f.authErr[token]; err != nil {
		return "", err
	}
	uuid := f.tokens[token]
	if uuid == "" {
		return "", inter.ErrInvalidToken
	}
	return uuid, nil
}

func (f *fakeRegistry) UpdateDeviceAuthenticateStatus(uuid string, status inter.AuthenticateStatusType) (string, error) {
	meta := f.metas[uuid]
	meta.AuthenticateStatus = status
	if status == inter.Authenticated {
		meta.Token = "token-" + uuid
	}
	f.metas[uuid] = meta
	return meta.Token, nil
}

func (f *fakeRegistry) RefreshToken(uuid string) (string, error) { return "token-" + uuid, nil }
func (f *fakeRegistry) RevokeToken(uuid string) error            { return nil }

func (f *fakeRegistry) GetDeviceMetadata(uuid string) (inter.DeviceMetadata, error) {
	if err := f.metadataErrors[uuid]; err != nil {
		return inter.DeviceMetadata{}, err
	}
	meta, ok := f.metas[uuid]
	if !ok {
		return inter.DeviceMetadata{}, inter.ErrDeviceNotFound
	}
	return meta, nil
}

func (f *fakeRegistry) ApproveDevice(uuid string) error { return nil }
func (f *fakeRegistry) RejectDevice(uuid string) error  { return nil }
func (f *fakeRegistry) UnblockDevice(uuid string) error { return nil }
func (f *fakeRegistry) DeleteDevice(uuid string) error  { return nil }
func (f *fakeRegistry) ListDevices(status *inter.AuthenticateStatusType, page, size int) ([]inter.DeviceRecord, error) {
	return nil, nil
}
func (f *fakeRegistry) ListDevicesByScope(scope inter.Scope, status *inter.AuthenticateStatusType, page, size int) ([]inter.DeviceRecord, error) {
	return nil, nil
}
func (f *fakeRegistry) GetDeviceMetadataByScope(scope inter.Scope, uuid string) (inter.DeviceMetadata, error) {
	return f.GetDeviceMetadata(uuid)
}

type fakePresence struct {
	heartbeats []string
}

func (f *fakePresence) HandleHeartbeat(uuid string) { f.heartbeats = append(f.heartbeats, uuid) }
func (f *fakePresence) QueryDeviceStatus(uuid string) (inter.DeviceStatus, error) {
	return inter.StatusOnline, nil
}

type fakeTelemetry struct {
	metrics []struct {
		uuid   string
		points []inter.MetricPoint
	}
	logs []struct {
		uuid string
		data inter.LogUploadData
	}
	events []struct {
		uuid    string
		payload []byte
	}
	errors []struct {
		uuid    string
		payload []byte
	}
	err error
}

func (f *fakeTelemetry) IngestMetrics(uuid string, points []inter.MetricPoint) error {
	if f.err != nil {
		return f.err
	}
	f.metrics = append(f.metrics, struct {
		uuid   string
		points []inter.MetricPoint
	}{uuid: uuid, points: points})
	return nil
}

func (f *fakeTelemetry) IngestLog(uuid string, data inter.LogUploadData) error {
	if f.err != nil {
		return f.err
	}
	f.logs = append(f.logs, struct {
		uuid string
		data inter.LogUploadData
	}{uuid: uuid, data: data})
	return nil
}

func (f *fakeTelemetry) IngestEvent(uuid string, payload []byte) error {
	if f.err != nil {
		return f.err
	}
	f.events = append(f.events, struct {
		uuid    string
		payload []byte
	}{uuid: uuid, payload: payload})
	return nil
}

func (f *fakeTelemetry) IngestDeviceError(uuid string, payload []byte) error {
	if f.err != nil {
		return f.err
	}
	f.errors = append(f.errors, struct {
		uuid    string
		payload []byte
	}{uuid: uuid, payload: payload})
	return nil
}

type fakeDownlink struct {
	queue    []inter.DownlinkMessage
	requeues []struct {
		uuid string
		msg  inter.DownlinkMessage
	}
	sent   []int64
	acked  []int64
	failed []struct {
		id  int64
		err string
	}
	err error
}

func (f *fakeDownlink) Enqueue(scope inter.Scope, uuid string, cmdID inter.CmdID, command string, payloadJSON []byte) (inter.DownlinkMessage, error) {
	return inter.DownlinkMessage{CommandID: 1, CmdID: cmdID, Payload: payloadJSON}, nil
}

func (f *fakeDownlink) PopDownlink(uuid string) (inter.DownlinkMessage, bool, error) {
	if f.err != nil {
		return inter.DownlinkMessage{}, false, f.err
	}
	if len(f.queue) == 0 {
		return inter.DownlinkMessage{}, false, nil
	}
	msg := f.queue[0]
	f.queue = f.queue[1:]
	return msg, true, nil
}

func (f *fakeDownlink) Requeue(uuid string, message inter.DownlinkMessage) error {
	if f.err != nil {
		return f.err
	}
	f.requeues = append(f.requeues, struct {
		uuid string
		msg  inter.DownlinkMessage
	}{uuid: uuid, msg: message})
	return nil
}

func (f *fakeDownlink) MarkSent(commandID int64) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, commandID)
	return nil
}

func (f *fakeDownlink) MarkAcked(commandID int64) error {
	if f.err != nil {
		return f.err
	}
	f.acked = append(f.acked, commandID)
	return nil
}

func (f *fakeDownlink) MarkFailed(commandID int64, errorText string) error {
	if f.err != nil {
		return f.err
	}
	f.failed = append(f.failed, struct {
		id  int64
		err string
	}{id: commandID, err: errorText})
	return nil
}

type fakeTenantResolver struct {
	tenants map[string]string
	err     error
}

func (f fakeTenantResolver) ResolveDeviceTenant(uuid string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.tenants[uuid], nil
}

func newTestCoreService() (*CoreService, *fakeRegistry, *fakePresence, *fakeTelemetry, *fakeDownlink) {
	registry := newFakeRegistry()
	presence := &fakePresence{}
	telemetry := &fakeTelemetry{}
	downlink := &fakeDownlink{}
	resolver := fakeTenantResolver{tenants: map[string]string{"dev-1": "tenant-a", "dev-2": "tenant-b", "uuid-SN1AA:BB": "tenant-new"}}
	return NewCoreService(registry, presence, telemetry, downlink, resolver), registry, presence, telemetry, downlink
}

func TestAuthenticateDeviceAcceptedAndFallbackIdentityToken(t *testing.T) {
	svc, registry, _, _, _ := newTestCoreService()
	registry.tokens["tok-1"] = "dev-1"
	registry.metas["dev-1"] = inter.DeviceMetadata{Name: "Device 1", SerialNumber: "SN1", MACAddress: "AA:BB", Token: "tok-1", AuthenticateStatus: inter.Authenticated}

	resp, err := svc.AuthenticateDevice(context.Background(), connect.NewRequest(&ingressv1.AuthenticateDeviceRequest{
		Identities: []*ingressv1.DeviceIdentity{{Type: "token", Value: " tok-1 "}},
	}))
	if err != nil {
		t.Fatalf("AuthenticateDevice failed: %v", err)
	}
	if resp.Msg.GetStatus() != ingressv1.AuthStatus_AUTH_STATUS_ACCEPTED || resp.Msg.GetUuid() != "dev-1" || resp.Msg.GetTenantId() != "tenant-a" {
		t.Fatalf("unexpected auth response: %+v", resp.Msg)
	}
	if registry.lastAuthToken != "tok-1" {
		t.Fatalf("expected identity token fallback, got %q", registry.lastAuthToken)
	}
	if resp.Msg.GetDevice().GetSerialNumber() != "SN1" || resp.Msg.GetDevice().GetLabels()["tenant_id"] != "tenant-a" {
		t.Fatalf("unexpected device descriptor: %+v", resp.Msg.GetDevice())
	}
}

func TestAuthenticateDeviceMapsBusinessStatuses(t *testing.T) {
	svc, registry, _, _, _ := newTestCoreService()
	registry.authErr["pending"] = inter.ErrDevicePending
	registry.authErr["refused"] = inter.ErrDeviceRefused
	registry.authErr["invalid"] = inter.ErrInvalidToken

	cases := []struct {
		token string
		want  ingressv1.AuthStatus
	}{
		{token: "", want: ingressv1.AuthStatus_AUTH_STATUS_REJECTED},
		{token: "pending", want: ingressv1.AuthStatus_AUTH_STATUS_PENDING},
		{token: "refused", want: ingressv1.AuthStatus_AUTH_STATUS_REJECTED},
		{token: "invalid", want: ingressv1.AuthStatus_AUTH_STATUS_UNKNOWN},
	}
	for _, tc := range cases {
		resp, err := svc.AuthenticateDevice(context.Background(), connect.NewRequest(&ingressv1.AuthenticateDeviceRequest{Credentials: []*ingressv1.Credential{{Type: "token", Value: tc.token}}}))
		if err != nil {
			t.Fatalf("AuthenticateDevice(%q) returned error: %v", tc.token, err)
		}
		if resp.Msg.GetStatus() != tc.want {
			t.Fatalf("AuthenticateDevice(%q) status=%s want %s", tc.token, resp.Msg.GetStatus(), tc.want)
		}
	}
}

func TestRegisterDeviceCreatesPendingAndHandlesExistingStatuses(t *testing.T) {
	svc, registry, _, _, _ := newTestCoreService()
	registry.generatedUUID = "uuid-SN1AA:BB"

	resp, err := svc.RegisterDevice(context.Background(), connect.NewRequest(&ingressv1.RegisterDeviceRequest{Device: &ingressv1.DeviceDescriptor{Name: "New", SerialNumber: "SN1", MacAddress: "AA:BB", FirmwareVersion: "fw1"}}))
	if err != nil {
		t.Fatalf("RegisterDevice new failed: %v", err)
	}
	if resp.Msg.GetStatus() != ingressv1.RegistrationStatus_REGISTRATION_STATUS_PENDING || resp.Msg.GetUuid() != "uuid-SN1AA:BB" || resp.Msg.GetTenantId() != "tenant-new" {
		t.Fatalf("unexpected pending response: %+v", resp.Msg)
	}
	if len(registry.registered) != 1 || registry.registered[0].SWVersion != "fw1" {
		t.Fatalf("registration metadata not mapped: %+v", registry.registered)
	}

	registry.metas["uuid-SN1AA:BB"] = inter.DeviceMetadata{Name: "Approved", SerialNumber: "SN1", MACAddress: "AA:BB", Token: "issued-token", AuthenticateStatus: inter.Authenticated}
	resp, err = svc.RegisterDevice(context.Background(), connect.NewRequest(&ingressv1.RegisterDeviceRequest{Device: &ingressv1.DeviceDescriptor{SerialNumber: "SN1", MacAddress: "AA:BB"}}))
	if err != nil {
		t.Fatalf("RegisterDevice existing failed: %v", err)
	}
	if resp.Msg.GetStatus() != ingressv1.RegistrationStatus_REGISTRATION_STATUS_ACCEPTED || resp.Msg.GetCredential().GetValue() != "issued-token" {
		t.Fatalf("unexpected accepted response: %+v", resp.Msg)
	}

	registry.metas["uuid-SN1AA:BB"] = inter.DeviceMetadata{SerialNumber: "SN1", MACAddress: "AA:BB", AuthenticateStatus: inter.AuthenticateRefuse}
	resp, err = svc.RegisterDevice(context.Background(), connect.NewRequest(&ingressv1.RegisterDeviceRequest{Device: &ingressv1.DeviceDescriptor{SerialNumber: "SN1", MacAddress: "AA:BB"}}))
	if err != nil {
		t.Fatalf("RegisterDevice refused failed: %v", err)
	}
	if resp.Msg.GetStatus() != ingressv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED {
		t.Fatalf("unexpected refused response: %+v", resp.Msg)
	}
}

func TestRegisterDeviceRejectsMissingIdentityAndReportsInitError(t *testing.T) {
	svc, registry, _, _, _ := newTestCoreService()
	resp, err := svc.RegisterDevice(context.Background(), connect.NewRequest(&ingressv1.RegisterDeviceRequest{Device: &ingressv1.DeviceDescriptor{Name: "nameless"}}))
	if err != nil {
		t.Fatalf("RegisterDevice missing identity returned rpc error: %v", err)
	}
	if resp.Msg.GetStatus() != ingressv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED {
		t.Fatalf("expected rejected response, got %+v", resp.Msg)
	}

	registry.generatedUUID = "uuid-init-error"
	registry.registerErr = errors.New("db down")
	_, err = svc.RegisterDevice(context.Background(), connect.NewRequest(&ingressv1.RegisterDeviceRequest{Device: &ingressv1.DeviceDescriptor{SerialNumber: "SN"}}))
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("expected internal init error, got %v code=%s", err, connect.CodeOf(err))
	}
}

func TestReportHeartbeatUsesUUIDOrPrimaryIdentity(t *testing.T) {
	svc, _, presence, _, _ := newTestCoreService()
	resp, err := svc.ReportHeartbeat(context.Background(), connect.NewRequest(&ingressv1.ReportHeartbeatRequest{PrimaryIdentity: &ingressv1.DeviceIdentity{Type: "uuid", Value: "dev-1"}}))
	if err != nil {
		t.Fatalf("ReportHeartbeat failed: %v", err)
	}
	if resp.Msg.GetUuid() != "dev-1" || resp.Msg.GetTenantId() != "tenant-a" || resp.Msg.GetAvailability() != ingressv1.DeviceAvailability_DEVICE_AVAILABILITY_ONLINE {
		t.Fatalf("unexpected heartbeat response: %+v", resp.Msg)
	}
	if len(presence.heartbeats) != 1 || presence.heartbeats[0] != "dev-1" {
		t.Fatalf("heartbeat not recorded: %+v", presence.heartbeats)
	}

	_, err = svc.ReportHeartbeat(context.Background(), connect.NewRequest(&ingressv1.ReportHeartbeatRequest{}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected invalid argument for missing uuid, got %v code=%s", err, connect.CodeOf(err))
	}
}

func TestIngestEventsWritesMetricsLogsRawAndCommandReceipts(t *testing.T) {
	svc, _, _, telemetry, downlink := newTestCoreService()
	now := timestamppb.New(time.Unix(1700000000, 123000000))
	value := &ingressv1.Value{Kind: &ingressv1.Value_NumberValue{NumberValue: 21.5}}
	jsonRaw, _ := structpb.NewStruct(map[string]any{"kind": "door", "open": true})

	resp, err := svc.IngestEvents(context.Background(), connect.NewRequest(&ingressv1.IngestEventsRequest{AllowPartialSuccess: true, Events: []*ingressv1.CanonicalDeviceEvent{
		{
			EventId: "metrics-1",
			Device:  &ingressv1.DeviceDescriptor{Uuid: "dev-1"},
			Metrics: []*ingressv1.MetricPoint{{Value: value, LegacyMetricType: 1, ObservedAt: now}},
			Logs:    []*ingressv1.LogRecord{{Level: ingressv1.LogLevel_LOG_LEVEL_WARN, Message: "battery low", ObservedAt: now}},
			CommandReceipt: &ingressv1.CommandReceipt{
				CommandId: 99,
				Status:    ingressv1.CommandStatus_COMMAND_STATUS_ACKED,
			},
		},
		{
			EventId:   "event-json",
			Device:    &ingressv1.DeviceDescriptor{Uuid: "dev-1"},
			EventType: ingressv1.EventType_EVENT_TYPE_DEVICE_EVENT,
			Raw:       &ingressv1.RawPayload{ContentType: "application/json", Json: jsonRaw},
		},
		{
			EventId:   "error-text",
			Device:    &ingressv1.DeviceDescriptor{Uuid: "dev-2"},
			EventType: ingressv1.EventType_EVENT_TYPE_DEVICE_ERROR,
			Raw:       &ingressv1.RawPayload{ContentType: "text/plain", Text: "boom"},
		},
	}}))
	if err != nil {
		t.Fatalf("IngestEvents failed: %v", err)
	}
	if len(resp.Msg.GetResults()) != 3 || !resp.Msg.GetResults()[0].GetSuccess() || !resp.Msg.GetResults()[1].GetSuccess() || !resp.Msg.GetResults()[2].GetSuccess() {
		t.Fatalf("unexpected ingest response: %+v", resp.Msg)
	}
	if len(telemetry.metrics) != 1 || telemetry.metrics[0].uuid != "dev-1" || len(telemetry.metrics[0].points) != 1 || telemetry.metrics[0].points[0].Type != 1 || telemetry.metrics[0].points[0].Value != float32(21.5) {
		t.Fatalf("unexpected metrics ingest: %+v", telemetry.metrics)
	}
	if len(telemetry.logs) != 1 || telemetry.logs[0].data.Level != inter.LogLevelWarn || telemetry.logs[0].data.Message != "battery low" {
		t.Fatalf("unexpected logs ingest: %+v", telemetry.logs)
	}
	if len(downlink.acked) != 1 || downlink.acked[0] != 99 {
		t.Fatalf("expected ack receipt to update command, got %+v", downlink.acked)
	}
	if len(telemetry.events) != 1 || !strings.Contains(string(telemetry.events[0].payload), "door") {
		t.Fatalf("unexpected raw event ingest: %+v", telemetry.events)
	}
	if len(telemetry.errors) != 1 || string(telemetry.errors[0].payload) != "boom" {
		t.Fatalf("unexpected error ingest: %+v", telemetry.errors)
	}
}

func TestIngestEventsPartialAndStopOnFirstFailure(t *testing.T) {
	svc, _, _, telemetry, _ := newTestCoreService()
	telemetry.err = errors.New("store down")

	resp, err := svc.IngestEvents(context.Background(), connect.NewRequest(&ingressv1.IngestEventsRequest{AllowPartialSuccess: false, Events: []*ingressv1.CanonicalDeviceEvent{
		{EventId: "missing"},
		{EventId: "skipped", Device: &ingressv1.DeviceDescriptor{Uuid: "dev-1"}},
	}}))
	if err != nil {
		t.Fatalf("IngestEvents missing uuid should return partial response, got error: %v", err)
	}
	if len(resp.Msg.GetResults()) != 1 || resp.Msg.GetResults()[0].GetErrorCode() != "uuid_required" {
		t.Fatalf("unexpected stop response: %+v", resp.Msg)
	}

	resp, err = svc.IngestEvents(context.Background(), connect.NewRequest(&ingressv1.IngestEventsRequest{AllowPartialSuccess: true, Events: []*ingressv1.CanonicalDeviceEvent{
		{EventId: "bad", Device: &ingressv1.DeviceDescriptor{Uuid: "dev-1"}, Metrics: []*ingressv1.MetricPoint{{Value: &ingressv1.Value{Kind: &ingressv1.Value_NumberValue{NumberValue: 1}}}}},
		{EventId: "missing"},
	}}))
	if err != nil {
		t.Fatalf("IngestEvents partial failed: %v", err)
	}
	if len(resp.Msg.GetResults()) != 2 || resp.Msg.GetResults()[0].GetSuccess() || resp.Msg.GetResults()[0].GetTenantId() != "tenant-a" || resp.Msg.GetResults()[1].GetErrorCode() != "uuid_required" {
		t.Fatalf("unexpected partial response: %+v", resp.Msg)
	}
}

func TestPullCommandsMapsDownlinkMessages(t *testing.T) {
	svc, _, _, _, downlink := newTestCoreService()
	downlink.queue = []inter.DownlinkMessage{
		{CommandID: 1, CmdID: inter.CmdConfigPush, Payload: []byte(`{"cfg":1}`)},
		{CommandID: 2, CmdID: inter.CmdActionExec, Payload: []byte(`{"op":"reboot"}`)},
	}

	resp, err := svc.PullCommands(context.Background(), connect.NewRequest(&ingressv1.PullCommandsRequest{PrimaryIdentity: &ingressv1.DeviceIdentity{Type: "uuid", Value: "dev-1"}, MaxCount: 5}))
	if err != nil {
		t.Fatalf("PullCommands failed: %v", err)
	}
	if len(resp.Msg.GetCommands()) != 2 {
		t.Fatalf("expected 2 commands, got %+v", resp.Msg.GetCommands())
	}
	cmd := resp.Msg.GetCommands()[0]
	if cmd.GetCommandId() != 1 || cmd.GetTenantId() != "tenant-a" || cmd.GetUuid() != "dev-1" || cmd.GetOperation() != "config_push" || cmd.GetProtocolCommandCode() != uint32(inter.CmdConfigPush) || string(cmd.GetPayload().GetBody()) != `{"cfg":1}` {
		t.Fatalf("unexpected command mapping: %+v", cmd)
	}
	if got := cmd.GetAdapterOptions().GetFields()["command_code"].GetNumberValue(); got != float64(inter.CmdConfigPush) {
		t.Fatalf("unexpected command_code option: %v", got)
	}

	_, err = svc.PullCommands(context.Background(), connect.NewRequest(&ingressv1.PullCommandsRequest{}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected invalid argument for missing uuid, got %v code=%s", err, connect.CodeOf(err))
	}
}

func TestUpdateCommandStatusSentAckedFailedAndRequeued(t *testing.T) {
	svc, _, _, _, downlink := newTestCoreService()
	statuses := []struct {
		status ingressv1.CommandStatus
		id     int64
		err    string
	}{
		{status: ingressv1.CommandStatus_COMMAND_STATUS_SENT, id: 10},
		{status: ingressv1.CommandStatus_COMMAND_STATUS_ACKED, id: 11},
		{status: ingressv1.CommandStatus_COMMAND_STATUS_FAILED, id: 12, err: "device rejected"},
	}
	for _, item := range statuses {
		resp, err := svc.UpdateCommandStatus(context.Background(), connect.NewRequest(&ingressv1.UpdateCommandStatusRequest{CommandId: item.id, Status: item.status, ErrorText: item.err}))
		if err != nil {
			t.Fatalf("UpdateCommandStatus(%s) failed: %v", item.status, err)
		}
		if !resp.Msg.GetSuccess() || resp.Msg.GetStatus() != item.status {
			t.Fatalf("unexpected update response: %+v", resp.Msg)
		}
	}
	if len(downlink.sent) != 1 || downlink.sent[0] != 10 || len(downlink.acked) != 1 || downlink.acked[0] != 11 || len(downlink.failed) != 1 || downlink.failed[0].id != 12 || downlink.failed[0].err != "device rejected" {
		t.Fatalf("unexpected status calls: sent=%+v acked=%+v failed=%+v", downlink.sent, downlink.acked, downlink.failed)
	}

	resp, err := svc.UpdateCommandStatus(context.Background(), connect.NewRequest(&ingressv1.UpdateCommandStatusRequest{
		CommandId: 21,
		Status:    ingressv1.CommandStatus_COMMAND_STATUS_REQUEUED,
		Command: &ingressv1.CanonicalCommand{
			Uuid:                "dev-1",
			ProtocolCommandCode: uint32(inter.CmdActionExec),
			Payload:             &ingressv1.RawPayload{Text: "turn-on"},
		},
	}))
	if err != nil {
		t.Fatalf("UpdateCommandStatus requeue failed: %v", err)
	}
	if !resp.Msg.GetSuccess() || len(downlink.requeues) != 1 || downlink.requeues[0].uuid != "dev-1" || downlink.requeues[0].msg.CommandID != 21 || downlink.requeues[0].msg.CmdID != inter.CmdActionExec || string(downlink.requeues[0].msg.Payload) != "turn-on" {
		t.Fatalf("unexpected requeue response/calls: resp=%+v requeues=%+v", resp.Msg, downlink.requeues)
	}
}

func TestUpdateCommandStatusRejectsInvalidRequests(t *testing.T) {
	svc, _, _, _, _ := newTestCoreService()
	cases := []*ingressv1.UpdateCommandStatusRequest{
		{Status: ingressv1.CommandStatus_COMMAND_STATUS_SENT},
		{CommandId: 1, Status: ingressv1.CommandStatus_COMMAND_STATUS_UNSPECIFIED},
		{CommandId: 2, Status: ingressv1.CommandStatus_COMMAND_STATUS_REQUEUED, ProtocolCommandCode: uint32(inter.CmdActionExec), Raw: &ingressv1.RawPayload{Body: []byte("x")}},
		{CommandId: 3, Status: ingressv1.CommandStatus_COMMAND_STATUS_REQUEUED, Uuid: "dev-1", Raw: &ingressv1.RawPayload{Body: []byte("x")}},
		{CommandId: 4, Status: ingressv1.CommandStatus_COMMAND_STATUS_REQUEUED, Uuid: "dev-1", ProtocolCommandCode: uint32(inter.CmdActionExec)},
	}
	for _, req := range cases {
		_, err := svc.UpdateCommandStatus(context.Background(), connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("expected invalid argument for %+v, got %v code=%s", req, err, connect.CodeOf(err))
		}
	}
}

func TestResolveTenantFallbacksToLegacy(t *testing.T) {
	registry := newFakeRegistry()
	svc := NewCoreService(registry, &fakePresence{}, &fakeTelemetry{}, &fakeDownlink{}, fakeTenantResolver{err: errors.New("not found")})
	resp, err := svc.ReportHeartbeat(context.Background(), connect.NewRequest(&ingressv1.ReportHeartbeatRequest{Uuid: "missing"}))
	if err != nil {
		t.Fatalf("ReportHeartbeat failed: %v", err)
	}
	if resp.Msg.GetTenantId() != inter.DefaultTenantID {
		t.Fatalf("expected default tenant fallback, got %q", resp.Msg.GetTenantId())
	}
}
