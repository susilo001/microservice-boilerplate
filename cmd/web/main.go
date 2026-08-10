package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	apiroute "github.com/boilerplate/microservice-boilerplate/api/http/v1/route"
	"github.com/boilerplate/microservice-boilerplate/config"
	_ "github.com/boilerplate/microservice-boilerplate/docs"
	analyticshttp "github.com/boilerplate/microservice-boilerplate/internal/analytics/delivery/http"
	analyticsmessaging "github.com/boilerplate/microservice-boilerplate/internal/analytics/gateway/messaging"
	analyticsusecase "github.com/boilerplate/microservice-boilerplate/internal/analytics/usecase"
	"github.com/boilerplate/microservice-boilerplate/internal/product/cache"
	producthttp "github.com/boilerplate/microservice-boilerplate/internal/product/delivery/http"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/gateway/messaging"
	"github.com/boilerplate/microservice-boilerplate/internal/product/repository"
	"github.com/boilerplate/microservice-boilerplate/internal/product/usecase"
	"github.com/boilerplate/microservice-boilerplate/pkg/databases"
	pkgkafka "github.com/boilerplate/microservice-boilerplate/pkg/kafka"
	pkglogger "github.com/boilerplate/microservice-boilerplate/pkg/logger"
	pkgredis "github.com/boilerplate/microservice-boilerplate/pkg/redis"
	"github.com/boilerplate/microservice-boilerplate/pkg/telemetry"
)

// @title           Product Microservice API
// @version         1.0
// @description     Clean Architecture Go microservice for product management.
//
// @contact.email   susiloodie@gmail.com
//
// @host            localhost:8003
// @BasePath        /
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
	otelShutdown, err := telemetry.InitProvider(ctx, cfg.OTelServiceName)
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

	// database
	db, err := databases.NewDB(databases.Config{
		Driver:          cfg.DB.Driver,
		DSN:             cfg.DB.DSN,
		ReplicaDSNs:     cfg.DB.ReplicaDSNs,
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.DB.ConnMaxIdleTime,
	})
	if err != nil {
		logger.Fatal("init db", zap.Error(err))
	}
	if err := db.AutoMigrate(&entity.Product{}); err != nil {
		logger.Fatal("auto migrate", zap.Error(err))
	}

	// redis
	rdb := pkgredis.NewClient(pkgredis.Config{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		MaxRetries:   cfg.Redis.MaxRetries,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})

	// kafka
	kafkaWriter := pkgkafka.NewWriter(pkgkafka.WriterConfig{
		Brokers:      cfg.Kafka.Brokers,
		BatchSize:    cfg.Kafka.BatchSize,
		BatchTimeout: cfg.Kafka.BatchTimeout,
		RequiredAcks: cfg.Kafka.RequiredAcks,
		Async:        cfg.Kafka.Async,
		Compression:  cfg.Kafka.Compression,
	})
	defer kafkaWriter.Close()

	// infrastructure — product domain
	productRepo := repository.NewProductRepository(db)
	productCache := cache.NewProductCache(rdb)
	productPublisher := messaging.NewProductPublisher(kafkaWriter)

	// use cases — product domain
	getProductUC := usecase.NewGetProductUseCase(productRepo, productCache)
	listProductsUC := usecase.NewListProductsUseCase(productRepo)
	createProductUC := usecase.NewCreateProductUseCase(productRepo, productPublisher)
	updateProductUC := usecase.NewUpdateProductUseCase(productRepo, productCache, productPublisher)
	deleteProductUC := usecase.NewDeleteProductUseCase(productRepo, productCache)

	// infrastructure — analytics domain (ingest path only; no DB pool here,
	// the raw pgxpool belongs to cmd/worker's consumer, not the web server)
	analyticsPublisher := analyticsmessaging.NewEventPublisher(kafkaWriter, cfg.Analytics.Topic)
	publishEventUC := analyticsusecase.NewPublishEventUseCase(analyticsPublisher)
	ingestHandler := analyticshttp.NewIngestHandler(publishEventUC)

	// delivery
	handler := producthttp.NewProductHandler(getProductUC, listProductsUC, createProductUC, updateProductUC, deleteProductUC)
	router := apiroute.NewRouter(handler, ingestHandler, logger, cfg.OTelServiceName)

	srv := &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: router,
	}

	go func() {
		logger.Info("server starting", zap.String("port", cfg.AppPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown", zap.Error(err))
	}
}
