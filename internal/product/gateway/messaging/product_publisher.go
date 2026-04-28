package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/event"
)

type productPublisher struct {
	writer *kafka.Writer
	tracer trace.Tracer
}

func NewProductPublisher(writer *kafka.Writer) event.Publisher {
	return &productPublisher{
		writer: writer,
		tracer: otel.Tracer("product-service/kafka"),
	}
}

func (p *productPublisher) Publish(ctx context.Context, e event.Event) error {
	ctx, span := p.tracer.Start(ctx, "kafka.publish."+e.EventName())
	defer span.End()

	payload, err := json.Marshal(e)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("marshal event %s: %w", e.EventName(), err)
	}

	headers := make(kafkaHeaderCarrier, 0)
	otel.GetTextMapPropagator().Inject(ctx, &headers)

	msg := kafka.Message{
		Topic:   e.EventName(),
		Value:   payload,
		Headers: []kafka.Header(headers),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("publish event %s: %w", e.EventName(), err)
	}
	return nil
}

// kafkaHeaderCarrier adapts []kafka.Header to propagation.TextMapCarrier.
type kafkaHeaderCarrier []kafka.Header

func (c *kafkaHeaderCarrier) Get(key string) string {
	for _, h := range *c {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *kafkaHeaderCarrier) Set(key, value string) {
	*c = append(*c, kafka.Header{Key: key, Value: []byte(value)})
}

func (c *kafkaHeaderCarrier) Keys() []string {
	keys := make([]string, len(*c))
	for i, h := range *c {
		keys[i] = h.Key
	}
	return keys
}

var _ propagation.TextMapCarrier = (*kafkaHeaderCarrier)(nil)
