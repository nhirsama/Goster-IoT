package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/adapter"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/adapter/customtcp"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/coreclient"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/normalizer"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/server"
)

type App struct {
	cfg        config.Config
	logger     *slog.Logger
	server     *server.Server
	adapters   []adapter.Adapter
	normalizer normalizer.Normalizer
	core       coreclient.Client
}

type Option func(*App)

func WithServer(s *server.Server) Option {
	return func(a *App) { a.server = s }
}

func WithAdapters(adapters ...adapter.Adapter) Option {
	return func(a *App) { a.adapters = adapters }
}

func WithCoreClient(core coreclient.Client) Option {
	return func(a *App) { a.core = core }
}

func WithNormalizer(n normalizer.Normalizer) Option {
	return func(a *App) { a.normalizer = n }
}

func New(cfg config.Config, logger *slog.Logger, opts ...Option) *App {
	if logger == nil {
		logger = slog.Default()
	}
	cfg.Normalize()
	core := coreclient.NewRemote(cfg.Core, logger)
	n := normalizer.New(cfg.Service.InstanceID)
	a := &App{
		cfg:        cfg,
		logger:     logger,
		server:     server.New(cfg.Server, logger),
		normalizer: n,
		core:       core,
	}
	for _, opt := range opts {
		opt(a)
	}
	if a.core == nil {
		a.core = core
	}
	if a.normalizer == nil {
		a.normalizer = n
	}
	if a.adapters == nil {
		a.adapters = buildAdapters(cfg, logger, a.core, a.normalizer)
	}
	return a
}

func buildAdapters(cfg config.Config, logger *slog.Logger, core coreclient.Client, n normalizer.Normalizer) []adapter.Adapter {
	return []adapter.Adapter{
		customtcp.New(cfg.Adapters.CustomTCP, logger, customtcp.WithSourceInstance(cfg.Service.InstanceID), customtcp.WithCoreClient(core), customtcp.WithNormalizer(n)),
	}
}

func (a *App) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, len(a.adapters)+1)

	if a.server != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.server.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- err
			}
		}()
	}

	for _, ad := range a.adapters {
		adapterInstance := ad
		if adapterInstance == nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.logger.Info("启动 adapter", "name", adapterInstance.Name())
			if err := adapterInstance.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- err
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		cancel()
		<-done
		return ctx.Err()
	case err := <-errCh:
		cancel()
		<-done
		return err
	case <-done:
		return nil
	}
}
