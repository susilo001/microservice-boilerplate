package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/event"
	"github.com/boilerplate/microservice-boilerplate/pkg/retry"
)

type productPublisher struct {
	writer       messageWriter
	tracer       trace.Tracer
	errorCounter metric.Int64Counter
}

type messageWriter interface {
	WriteMessages(context.Context, ...kafka.Message) error
}

func NewProductPublisher(writer messageWriter) event.Publisher {
	errorCounter, _ := otel.Meter("product-service/kafka").Int64Counter(
		"product.kafka.publish.errors",
		metric.WithDescription("Number of failed product publish operations after retries"),
	)
	return &productPublisher{
		writer:       writer,
		tracer:       otel.Tracer("product-service/kafka"),
		errorCounter: errorCounter,
	}
}

func (p *productPublisher) Publish(ctx context.Context, e event.Event) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ctx, span := p.tracer.Start(ctx, "kafka.publish."+e.EventName())
	defer span.End()

	payload, err := json.Marshal(e)
	if err != nil {
		p.recordError(ctx, e.EventName(), "marshal")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("marshal event %s: %w", e.EventName(), err)
	}

	headers := make(kafkaHeaderCarrier, 0)
	otel.GetTextMapPropagator().Inject(ctx, &headers)

	msg := kafka.Message{
		Topic:   e.EventName(),
		Value:   payload,
		Headers: []kafka.Header(headers),
	}
	if aggregate, ok := e.(interface{ AggregateID() string }); ok {
		msg.Key = []byte(aggregate.AggregateID())
	}

	policy := retry.DefaultConfig()
	policy.Retryable = retryableKafkaError
	if err := retry.Do(ctx, policy, func() error {
		return p.writer.WriteMessages(ctx, msg)
	}); err != nil {
		p.recordError(ctx, e.EventName(), "write")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("publish event %s: %w", e.EventName(), err)
	}
	return nil
}

func retryableKafkaError(err error) bool {
	var writeErrors kafka.WriteErrors
	if errors.As(err, &writeErrors) {
		failed := false
		for _, writeErr := range writeErrors {
			if writeErr != nil {
				failed = true
				if !retryableKafkaError(writeErr) {
					return false
				}
			}
		}
		return failed
	}
	var kafkaError kafka.Error
	if errors.As(err, &kafkaError) {
		return kafkaError.Temporary()
	}
	return retry.IsTransient(err)
}

func (p *productPublisher) recordError(ctx context.Context, eventName, stage string) {
	p.errorCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("messaging.event.name", eventName),
		attribute.String("messaging.operation", stage),
	))
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
