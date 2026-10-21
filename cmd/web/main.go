package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/plugin/dbresolver"

	apiroute "github.com/boilerplate/microservice-boilerplate/api/http/v1/route"
	"github.com/boilerplate/microservice-boilerplate/config"
	_ "github.com/boilerplate/microservice-boilerplate/docs"
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
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	gin.SetMode(cfg.GinMode)

	logger, err := pkglogger.NewLogger()
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// telemetry — must shut down last
	otelShutdown, err := telemetry.InitProvider(ctx, cfg.OTelServiceName)
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
		return fmt.Errorf("init db: %w", err)
	}
	defer func() {
		if err := databases.Close(db); err != nil {
			logger.Error("close database pools", zap.Error(err))
		}
	}()
	if cfg.DBAutoMigrate {
		if err := db.AutoMigrate(&entity.Product{}); err != nil {
			return fmt.Errorf("auto migrate: %w", err)
		}
	}

	// redis
	rdb := pkgredis.NewClient(pkgredis.Config{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	defer func() {
		if err := rdb.Close(); err != nil {
			logger.Error("close redis", zap.Error(err))
		}
	}()

	// kafka
	kafkaWriter := pkgkafka.NewWriter(pkgkafka.WriterConfig{
		Brokers:      cfg.Kafka.Brokers,
		BatchSize:    cfg.Kafka.BatchSize,
		BatchTimeout: cfg.Kafka.BatchTimeout,
		RequiredAcks: cfg.Kafka.RequiredAcks,
		Async:        cfg.Kafka.Async,
		Compression:  cfg.Kafka.Compression,
	})
	defer func() {
		if err := kafkaWriter.Close(); err != nil {
			logger.Error("close Kafka writer", zap.Error(err))
		}
	}()

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

	// delivery
	handler := producthttp.NewProductHandler(getProductUC, listProductsUC, createProductUC, updateProductUC, deleteProductUC)
	router := apiroute.NewRouter(handler, logger, cfg.OTelServiceName, apiroute.Options{
		RequestTimeout: cfg.HTTPRequestTimeout,
		Ready: func(ctx context.Context) error {
			return db.WithContext(ctx).Clauses(dbresolver.Write).
				Exec("SELECT id, version FROM products WHERE 1 = 0").Error
		},
	})

	srv := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("server starting", zap.String("port", cfg.AppPort))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		return fmt.Errorf("serve HTTP: %w", err)
	}
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close()
		return fmt.Errorf("server shutdown: %w", err)
	}
	return nil
}
