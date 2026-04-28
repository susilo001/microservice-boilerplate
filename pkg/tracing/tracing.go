package tracing

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// IDs returns the trace ID and span ID from the active span in ctx.
// Both values are empty strings when ctx carries no valid span.
func IDs(ctx context.Context) (traceID, spanID string) {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.IsValid() {
		return "", ""
	}
	return sc.TraceID().String(), sc.SpanID().String()
}
