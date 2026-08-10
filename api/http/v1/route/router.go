package route

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"

	apimw "github.com/boilerplate/microservice-boilerplate/api/http/v1/middleware"
	analyticshttp "github.com/boilerplate/microservice-boilerplate/internal/analytics/delivery/http"
	producthttp "github.com/boilerplate/microservice-boilerplate/internal/product/delivery/http"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
)

func NewRouter(handler *producthttp.ProductHandler, ingestHandler *analyticshttp.IngestHandler, log *zap.Logger, serviceName string) *gin.Engine {
	r := gin.New()
	r.Use(apimw.RequestTimer())
	r.Use(apimw.CorrelationID())
	r.Use(otelgin.Middleware(serviceName))
	r.Use(apimw.TraceContext())
	r.Use(apimw.Logger(log))
	r.Use(apimw.ErrorHandler(func(err error) int {
		if errors.Is(err, entity.ErrProductNotFound) {
			return http.StatusNotFound
		}
		if errors.Is(err, entity.ErrInvalidSortField) || errors.Is(err, entity.ErrInvalidPreload) {
			return http.StatusBadRequest
		}
		return http.StatusInternalServerError
	}))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
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

		v1.POST("/events", ingestHandler.Ingest)
	}

	return r
}
