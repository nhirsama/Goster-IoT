package coreclient

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1"
	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1/ingressv1connect"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client interface {
	AuthenticateDevice(ctx context.Context, req *ingressv1.AuthenticateDeviceRequest) (*ingressv1.AuthenticateDeviceResponse, error)
	RegisterDevice(ctx context.Context, req *ingressv1.RegisterDeviceRequest) (*ingressv1.RegisterDeviceResponse, error)
	ReportHeartbeat(ctx context.Context, req *ingressv1.ReportHeartbeatRequest) (*ingressv1.ReportHeartbeatResponse, error)
	IngestEvents(ctx context.Context, req *ingressv1.IngestEventsRequest) (*ingressv1.IngestEventsResponse, error)
	PullCommands(ctx context.Context, req *ingressv1.PullCommandsRequest) (*ingressv1.PullCommandsResponse, error)
	UpdateCommandStatus(ctx context.Context, req *ingressv1.UpdateCommandStatusRequest) (*ingressv1.UpdateCommandStatusResponse, error)
}

type RemoteClient struct {
	cfg    config.CoreConfig
	logger *slog.Logger
	client ingressv1connect.ProtocolIngressCoreServiceClient
}

func NewRemote(cfg config.CoreConfig, logger *slog.Logger, opts ...connect.ClientOption) *RemoteClient {
	return NewRemoteWithHTTPClient(cfg, logger, http.DefaultClient, opts...)
}

func NewRemoteWithHTTPClient(cfg config.CoreConfig, logger *slog.Logger, httpClient connect.HTTPClient, opts ...connect.ClientOption) *RemoteClient {
	if logger == nil {
		logger = slog.Default()
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	clientOpts := make([]connect.ClientOption, 0, len(opts)+1)
	if strings.TrimSpace(cfg.Token) != "" {
		clientOpts = append(clientOpts, connect.WithInterceptors(bearerTokenInterceptor(cfg.Token)))
	}
	clientOpts = append(clientOpts, opts...)
	return &RemoteClient{
		cfg:    cfg,
		logger: logger,
		client: ingressv1connect.NewProtocolIngressCoreServiceClient(httpClient, cfg.Endpoint, clientOpts...),
	}
}

func NewRemoteWithGeneratedClient(cfg config.CoreConfig, logger *slog.Logger, client ingressv1connect.ProtocolIngressCoreServiceClient) *RemoteClient {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &RemoteClient{cfg: cfg, logger: logger, client: client}
}

func (c *RemoteClient) AuthenticateDevice(ctx context.Context, req *ingressv1.AuthenticateDeviceRequest) (*ingressv1.AuthenticateDeviceResponse, error) {
	if req == nil {
		return nil, errors.New("AuthenticateDeviceRequest 不能为空")
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.client.AuthenticateDevice(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func (c *RemoteClient) RegisterDevice(ctx context.Context, req *ingressv1.RegisterDeviceRequest) (*ingressv1.RegisterDeviceResponse, error) {
	if req == nil {
		return nil, errors.New("RegisterDeviceRequest 不能为空")
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.client.RegisterDevice(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func (c *RemoteClient) ReportHeartbeat(ctx context.Context, req *ingressv1.ReportHeartbeatRequest) (*ingressv1.ReportHeartbeatResponse, error) {
	if req == nil {
		return nil, errors.New("ReportHeartbeatRequest 不能为空")
	}
	if req.ObservedAt == nil {
		req.ObservedAt = timestamppb.Now()
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.client.ReportHeartbeat(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func (c *RemoteClient) IngestEvents(ctx context.Context, req *ingressv1.IngestEventsRequest) (*ingressv1.IngestEventsResponse, error) {
	if req == nil {
		return nil, errors.New("IngestEventsRequest 不能为空")
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.client.IngestEvents(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func (c *RemoteClient) PullCommands(ctx context.Context, req *ingressv1.PullCommandsRequest) (*ingressv1.PullCommandsResponse, error) {
	if req == nil {
		return nil, errors.New("PullCommandsRequest 不能为空")
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.client.PullCommands(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func (c *RemoteClient) UpdateCommandStatus(ctx context.Context, req *ingressv1.UpdateCommandStatusRequest) (*ingressv1.UpdateCommandStatusResponse, error) {
	if req == nil {
		return nil, errors.New("UpdateCommandStatusRequest 不能为空")
	}
	if req.ObservedAt == nil {
		req.ObservedAt = timestamppb.Now()
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.client.UpdateCommandStatus(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func (c *RemoteClient) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, c.cfg.Timeout)
}

func bearerTokenInterceptor(token string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("Authorization", "Bearer "+token)
			return next(ctx, req)
		}
	}
}

var _ Client = (*RemoteClient)(nil)
