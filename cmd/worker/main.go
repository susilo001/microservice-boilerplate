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
	analyticsmessaging "github.com/boilerplate/microservice-boilerplate/internal/analytics/gateway/messaging"
	analyticsrepo "github.com/boilerplate/microservice-boilerplate/internal/analytics/repository"
	"github.com/boilerplate/microservice-boilerplate/pkg/databases"
	pkgkafka "github.com/boilerplate/microservice-boilerplate/pkg/kafka"
	pkglogger "github.com/boilerplate/microservice-boilerplate/pkg/logger"
	"github.com/boilerplate/microservice-boilerplate/pkg/telemetry"
)

func main() {
	cfg := config.Load()

	logger, err := pkglogger.NewLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// telemetry — must shut down last
	otelShutdown, err := telemetry.InitProvider(ctx, cfg.OTelServiceName+"-worker")
	if err != nil {
		logger.Fatal("init otel", zap.Error(err))
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelShutdown(shutdownCtx); err != nil {
			logger.Error("otel shutdown", zap.Error(err))
		}
	}()

	// analytics domain's raw pgxpool — sole consumer of pkg/databases.NewRawPool
	pool, err := databases.NewRawPool(ctx, databases.RawConfig{
		DSN:      cfg.Analytics.DBDSN,
		MaxConns: cfg.Analytics.DBMaxConns,
	})
	if err != nil {
		logger.Fatal("init analytics db", zap.Error(err))
	}
	defer pool.Close()

	reader := pkgkafka.NewReader(pkgkafka.ReaderConfig{
		Brokers: cfg.Kafka.Brokers,
		Topic:   cfg.Analytics.Topic,
		GroupID: cfg.Analytics.ConsumerGroup,
	})
	defer reader.Close() //nolint:errcheck

	eventRepo := analyticsrepo.NewEventRepository(pool)
	consumer := analyticsmessaging.NewEventConsumer(reader, eventRepo, logger, cfg.Analytics.BatchSize, cfg.Analytics.BatchTimeout)

	go consumer.Run(ctx)

	logger.Info("worker started", zap.String("topic", cfg.Analytics.Topic), zap.String("group", cfg.Analytics.ConsumerGroup))
	<-ctx.Done()
	logger.Info("worker shutting down")
}
