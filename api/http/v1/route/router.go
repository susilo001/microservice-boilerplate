package route

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"

	apimw "github.com/boilerplate/microservice-boilerplate/api/http/v1/middleware"
	apiresp "github.com/boilerplate/microservice-boilerplate/api/http/v1/response"
	producthttp "github.com/boilerplate/microservice-boilerplate/internal/product/delivery/http"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	pkglogger "github.com/boilerplate/microservice-boilerplate/pkg/logger"
)

type Options struct {
	RequestTimeout time.Duration
	Ready          func(context.Context) error
}

func NewRouter(handler *producthttp.ProductHandler, log *zap.Logger, serviceName string, options Options) *gin.Engine {
	r := gin.New()
	r.Use(apimw.RequestTimer())
	r.Use(apimw.CorrelationID())
	r.Use(otelgin.Middleware(serviceName))
	r.Use(apimw.TraceContext())
	r.Use(apimw.Logger(log))
	r.Use(gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		pkglogger.FromContext(c.Request.Context(), log).Error("request panic", zap.Any("panic", recovered), zap.Stack("panic_stack"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, apiresp.NewError(
			c.GetString("correlationid"), apiresp.ContextTime(c, "tin"), time.Now(), "Internal Server Error"))
	}))
	r.Use(apimw.Deadline(options.RequestTimeout))
	r.Use(apimw.BodyLimit())
	r.Use(apimw.ErrorHandler(func(err error) int {
		if errors.Is(err, entity.ErrProductNotFound) {
			return http.StatusNotFound
		}
		if errors.Is(err, entity.ErrProductConflict) {
			return http.StatusConflict
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return http.StatusGatewayTimeout
		}
		if errors.Is(err, entity.ErrInvalidProduct) || errors.Is(err, entity.ErrInvalidSortField) || errors.Is(err, entity.ErrInvalidPreload) || errors.Is(err, entity.ErrInvalidProductQuery) {
			return http.StatusBadRequest
		}
		return http.StatusInternalServerError
	}))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		if options.Ready == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		if err := options.Ready(ctx); err != nil {
			pkglogger.FromContext(c.Request.Context(), log).Warn("readiness check failed", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	v1 := r.Group("/api/v1")
	{
		products := v1.Group("/products")
		{
			products.GET("", handler.List)
			products.GET("/:id", handler.GetByID)
			products.POST("", handler.Create)
			products.PUT("/:id", handler.Update)
			products.DELETE("/:id", handler.Delete)
		}
	}

	return r
}
