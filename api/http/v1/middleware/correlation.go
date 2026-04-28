package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CorrelationID reads X-Correlation-ID from the request header (or generates one),
// stores it in the context under "correlationid", and echoes it in the response header.
func CorrelationID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Correlation-ID")
		if id == "" {
			id = uuid.New().String()
		}
		c.Set("correlationid", id)
		c.Header("X-Correlation-ID", id)
		c.Next()
	}
}

// RequestTimer records the request-received timestamp in the context under "tin".
func RequestTimer() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("tin", time.Now())
		c.Next()
	}
}
