package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/boilerplate/microservice-boilerplate/config"
	pkglogger "github.com/boilerplate/microservice-boilerplate/pkg/logger"
	"github.com/boilerplate/microservice-boilerplate/pkg/telemetry"
)

func main() {
	cfg := config.Load()

	logger, err := pkglogger.NewLogger(cfg.Log)
	if err != nil {
		pkglogger.ReportStartupError(err)
		os.Exit(1)
	}
	logger = logger.With(zap.String("service", cfg.OTelServiceName+"-scheduler"))
	if err := run(cfg, logger); err != nil {
		logger.Error("application stopped", zap.Error(err))
		_ = logger.Sync()
		os.Exit(1)
	}
	_ = logger.Sync()
}

func run(cfg config.Config, logger *zap.Logger) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	logger.Debug("configuration validated")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// telemetry — must shut down last
	otelShutdown, err := telemetry.InitProvider(ctx, cfg.OTelServiceName+"-scheduler")
	if err != nil {
		return fmt.Errorf("init otel: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelShutdown(shutdownCtx); err != nil {
			logger.Error("otel shutdown", zap.Error(err))
		}
	}()

	// TODO: register scheduled jobs here.
	// Example pattern:
	//   scheduler := cron.New()
	//   scheduler.AddFunc("@every 1h", func() { /* job logic */ })
	//   scheduler.Start()
	//   defer scheduler.Stop()

	logger.Info("scheduler started")
	<-ctx.Done()
	logger.Info("scheduler shutting down")
	return nil
}
