package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Logger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
		}
		if traceID := c.GetString("trace_id"); traceID != "" {
			fields = append(fields,
				zap.String("trace_id", traceID),
				zap.String("span_id", c.GetString("span_id")),
			)
		}

		fields = append(fields, zap.String("correlationid", c.GetString("correlationid")))
		if len(c.Errors) > 0 {
			fields = append(fields, zap.Error(c.Errors.Last().Err))
		}
		switch {
		case c.Writer.Status() >= http.StatusInternalServerError:
			log.Error("request", fields...)
		case c.Writer.Status() >= http.StatusBadRequest:
			log.Warn("request", fields...)
		case c.Request.URL.Path == "/health" || c.Request.URL.Path == "/ready":
			log.Debug("request", fields...)
		default:
			log.Info("request", fields...)
		}
	}
}
