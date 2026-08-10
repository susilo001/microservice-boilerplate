package usecase

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/event"
	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/request"
)

var tracer = otel.Tracer("analytics/usecase")

type PublishEventUseCase interface {
	Execute(ctx context.Context, req request.IngestEventRequest) error
}

type publishEventUseCase struct {
	publisher event.Publisher
}

func NewPublishEventUseCase(publisher event.Publisher) PublishEventUseCase {
	return &publishEventUseCase{publisher: publisher}
}

// Execute validates and publishes to Kafka. It never touches Postgres —
// that is the single most important behavioral contract in this domain
// (AGENTS.md: "ingest handler never performs a synchronous Postgres call").
func (uc *publishEventUseCase) Execute(ctx context.Context, req request.IngestEventRequest) error {
	ctx, span := tracer.Start(ctx, "analytics.usecase.PublishEvent")
	defer span.End()
	span.SetAttributes(attribute.String("event.id", req.EventID))

	evt := entity.Event{
		EventID:    req.EventID,
		UserID:     req.UserID,
		EventType:  req.EventType,
		Payload:    req.Payload,
		OccurredAt: time.Now(),
	}
	if err := evt.Validate(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	wire := event.RawEventPublished{
		EventID:         evt.EventID,
		UserID:          evt.UserID,
		EventType:       evt.EventType,
		Payload:         evt.Payload,
		OccurredAtValue: evt.OccurredAt,
	}
	if err := uc.publisher.Publish(ctx, wire); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("publish event %s: %w", evt.EventID, err)
	}
	return nil
}
