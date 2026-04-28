package middleware

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

// TraceContext reads the active OTel span from the request context — either
// propagated from an upstream service via W3C traceparent/tracestate headers,
// or freshly created by otelgin when this is the trace origin — and stores
// trace_id and span_id in the Gin context for easy access in handlers and logs.
//
// Must be registered after otelgin.Middleware in the middleware chain.
func TraceContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		sc := trace.SpanFromContext(c.Request.Context()).SpanContext()
		if sc.IsValid() {
			c.Set("trace_id", sc.TraceID().String())
			c.Set("span_id", sc.SpanID().String())
		}
		c.Next()
	}
}
