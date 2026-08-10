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

	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/event"
)

type eventPublisher struct {
	writer *kafka.Writer
	topic  string
	tracer trace.Tracer
}

// NewEventPublisher publishes to a fixed topic (config.KAFKA_TOPIC) rather
// than the event's own name, so the ingest topic stays operator-configurable.
func NewEventPublisher(writer *kafka.Writer, topic string) event.Publisher {
	return &eventPublisher{
		writer: writer,
		topic:  topic,
		tracer: otel.Tracer("analytics/kafka"),
	}
}

func (p *eventPublisher) Publish(ctx context.Context, e event.Event) error {
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
		Topic:   p.topic,
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
