package coreclient

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1"
	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1/ingressv1connect"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
)

type fakeCoreService struct {
	t               *testing.T
	lastAuthHeader  string
	authReq         *ingressv1.AuthenticateDeviceRequest
	registerReq     *ingressv1.RegisterDeviceRequest
	heartbeatReq    *ingressv1.ReportHeartbeatRequest
	ingestReq       *ingressv1.IngestEventsRequest
	pullReq         *ingressv1.PullCommandsRequest
	updateStatusReq *ingressv1.UpdateCommandStatusRequest
}

func (f *fakeCoreService) AuthenticateDevice(ctx context.Context, req *connect.Request[ingressv1.AuthenticateDeviceRequest]) (*connect.Response[ingressv1.AuthenticateDeviceResponse], error) {
	f.lastAuthHeader = req.Header().Get("Authorization")
	f.authReq = req.Msg
	return connect.NewResponse(&ingressv1.AuthenticateDeviceResponse{Status: ingressv1.AuthStatus_AUTH_STATUS_ACCEPTED, Uuid: "dev-1", TenantId: "tenant-a"}), nil
}

func (f *fakeCoreService) RegisterDevice(ctx context.Context, req *connect.Request[ingressv1.RegisterDeviceRequest]) (*connect.Response[ingressv1.RegisterDeviceResponse], error) {
	f.registerReq = req.Msg
	return connect.NewResponse(&ingressv1.RegisterDeviceResponse{Status: ingressv1.RegistrationStatus_REGISTRATION_STATUS_PENDING, Uuid: "dev-2"}), nil
}

func (f *fakeCoreService) ReportHeartbeat(ctx context.Context, req *connect.Request[ingressv1.ReportHeartbeatRequest]) (*connect.Response[ingressv1.ReportHeartbeatResponse], error) {
	f.heartbeatReq = req.Msg
	return connect.NewResponse(&ingressv1.ReportHeartbeatResponse{Uuid: req.Msg.Uuid, Availability: ingressv1.DeviceAvailability_DEVICE_AVAILABILITY_ONLINE}), nil
}

func (f *fakeCoreService) IngestEvents(ctx context.Context, req *connect.Request[ingressv1.IngestEventsRequest]) (*connect.Response[ingressv1.IngestEventsResponse], error) {
	f.ingestReq = req.Msg
	return connect.NewResponse(&ingressv1.IngestEventsResponse{Results: []*ingressv1.EventIngestResult{{EventId: "evt-1", Success: true}}}), nil
}

func (f *fakeCoreService) PullCommands(ctx context.Context, req *connect.Request[ingressv1.PullCommandsRequest]) (*connect.Response[ingressv1.PullCommandsResponse], error) {
	f.pullReq = req.Msg
	return connect.NewResponse(&ingressv1.PullCommandsResponse{Commands: []*ingressv1.CanonicalCommand{{CommandId: 7, Operation: "config_push"}}}), nil
}

func (f *fakeCoreService) UpdateCommandStatus(ctx context.Context, req *connect.Request[ingressv1.UpdateCommandStatusRequest]) (*connect.Response[ingressv1.UpdateCommandStatusResponse], error) {
	f.updateStatusReq = req.Msg
	return connect.NewResponse(&ingressv1.UpdateCommandStatusResponse{Success: true, Status: req.Msg.Status}), nil
}

func newTestClient(t *testing.T, svc *fakeCoreService) (*RemoteClient, func()) {
	t.Helper()
	mux := http.NewServeMux()
	path, handler := ingressv1connect.NewProtocolIngressCoreServiceHandler(svc)
	mux.Handle(path, handler)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("tcp listener unavailable: %v", err)
	}
	server := httptest.NewUnstartedServer(mux)
	server.Listener = listener
	server.Start()
	client := NewRemote(config.CoreConfig{Endpoint: server.URL, Timeout: time.Second, Token: "secret"}, nil)
	return client, server.Close
}

func TestRemoteClientCallsAllRPCsAndAddsBearerToken(t *testing.T) {
	svc := &fakeCoreService{t: t}
	client, closeFn := newTestClient(t, svc)
	defer closeFn()
	ctx := context.Background()

	authResp, err := client.AuthenticateDevice(ctx, &ingressv1.AuthenticateDeviceRequest{Credentials: []*ingressv1.Credential{{Type: "token", Value: "dev-token"}}})
	if err != nil {
		t.Fatalf("AuthenticateDevice failed: %v", err)
	}
	if authResp.Uuid != "dev-1" || svc.lastAuthHeader != "Bearer secret" || svc.authReq.GetCredentials()[0].GetValue() != "dev-token" {
		t.Fatalf("unexpected auth result/header/req: resp=%+v header=%q req=%+v", authResp, svc.lastAuthHeader, svc.authReq)
	}

	if _, err := client.RegisterDevice(ctx, &ingressv1.RegisterDeviceRequest{Device: &ingressv1.DeviceDescriptor{Name: "new"}}); err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}
	if svc.registerReq.GetDevice().GetName() != "new" {
		t.Fatalf("register request not delivered: %+v", svc.registerReq)
	}

	if _, err := client.ReportHeartbeat(ctx, &ingressv1.ReportHeartbeatRequest{Uuid: "dev-1"}); err != nil {
		t.Fatalf("ReportHeartbeat failed: %v", err)
	}
	if svc.heartbeatReq.GetObservedAt() == nil {
		t.Fatal("ReportHeartbeat should fill observed_at")
	}

	if _, err := client.IngestEvents(ctx, &ingressv1.IngestEventsRequest{Events: []*ingressv1.CanonicalDeviceEvent{{EventId: "evt-1"}}}); err != nil {
		t.Fatalf("IngestEvents failed: %v", err)
	}
	if svc.ingestReq.GetEvents()[0].GetEventId() != "evt-1" {
		t.Fatalf("ingest request not delivered: %+v", svc.ingestReq)
	}

	pullResp, err := client.PullCommands(ctx, &ingressv1.PullCommandsRequest{Uuid: "dev-1", MaxCount: 1})
	if err != nil {
		t.Fatalf("PullCommands failed: %v", err)
	}
	if len(pullResp.GetCommands()) != 1 || svc.pullReq.GetUuid() != "dev-1" {
		t.Fatalf("unexpected pull response/req: resp=%+v req=%+v", pullResp, svc.pullReq)
	}

	if _, err := client.UpdateCommandStatus(ctx, &ingressv1.UpdateCommandStatusRequest{CommandId: 7, Status: ingressv1.CommandStatus_COMMAND_STATUS_SENT}); err != nil {
		t.Fatalf("UpdateCommandStatus failed: %v", err)
	}
	if svc.updateStatusReq.GetObservedAt() == nil || svc.updateStatusReq.GetStatus() != ingressv1.CommandStatus_COMMAND_STATUS_SENT {
		t.Fatalf("unexpected status req: %+v", svc.updateStatusReq)
	}
}

func TestRemoteClientRejectsNilRequests(t *testing.T) {
	client := NewRemote(config.CoreConfig{Endpoint: "http://127.0.0.1:1", Timeout: time.Second}, nil)
	if _, err := client.AuthenticateDevice(context.Background(), nil); err == nil {
		t.Fatal("expected nil auth request error")
	}
	if _, err := client.IngestEvents(context.Background(), nil); err == nil {
		t.Fatal("expected nil ingest request error")
	}
}
