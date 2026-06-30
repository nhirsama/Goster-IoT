package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/app"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("protocol-ingress 环境变量配置无效", "error", err)
		os.Exit(1)
	}

	logger.Info("protocol-ingress 启动", "service", cfg.Service.Name, "env", cfg.Service.Env, "instance", cfg.Service.InstanceID)
	if err := app.New(cfg, logger).Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("protocol-ingress 异常退出", "error", err)
		os.Exit(1)
	}
	logger.Info("protocol-ingress 正常关闭")
}
