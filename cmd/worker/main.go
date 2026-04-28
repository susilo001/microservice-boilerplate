package main

import (
	"context"
	"fmt"
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

	logger, err := pkglogger.NewLogger()
	if err != nil {
		panic(fmt.Sprintf("init logger: %v", err))
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

	// TODO: initialise Kafka consumers and wire domain dependencies here.
	// Example pattern:
	//   kafkaReader := pkgkafka.NewReader(cfg.KafkaBrokers, "product.created", "worker-group")
	//   defer kafkaReader.Close()
	//   consumer := messaging.NewProductConsumer(kafkaReader, ...)
	//   go consumer.Run(ctx)

	logger.Info("worker started")
	<-ctx.Done()
	logger.Info("worker shutting down")
}
