package messaging

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/event"
	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/repository"
)

var consumerTracer = otel.Tracer("analytics/consumer")

// EventConsumer reads raw-events, batches them (size OR timeout, via
// BatchAccumulator), and flushes to Postgres. Kafka offsets are committed
// only after InsertBatch durably succeeds — never before (AGENTS.md).
type EventConsumer struct {
	reader  *kafka.Reader
	repo    repository.EventRepository
	log     *zap.Logger
	batch   *BatchAccumulator
	pending []kafka.Message // uncommitted messages, same order as batch's events
	timeout time.Duration
}

func NewEventConsumer(reader *kafka.Reader, repo repository.EventRepository, log *zap.Logger, batchSize int, batchTimeout time.Duration) *EventConsumer {
	return &EventConsumer{
		reader:  reader,
		repo:    repo,
		log:     log,
		batch:   NewBatchAccumulator(batchSize, batchTimeout),
		timeout: batchTimeout,
	}
}

// Run blocks until ctx is cancelled. It polls with a per-read deadline no
// longer than the batch timeout, so a quiet topic still flushes on time.
func (c *EventConsumer) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			c.flush(context.Background(), "shutdown")
			return
		}

		readCtx, cancel := context.WithTimeout(ctx, c.timeout)
		msg, err := c.reader.FetchMessage(readCtx)
		cancel()

		if err != nil {
			if c.batch.DueByTimeout(time.Now()) {
				c.flush(ctx, "timeout")
			}
			continue
		}

		evt, err := decodeEvent(msg)
		if err != nil {
			c.log.Error("decode event", zap.Error(err))
			// ponytail: no dead-letter topic yet — malformed messages are
			// dropped after logging. Add dead-letter-events publish here
			// when that topic exists (AGENTS.md Good First Task #2).
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.log.Error("commit malformed message", zap.Error(err))
			}
			continue
		}

		c.pending = append(c.pending, msg)
		if shouldFlush := c.batch.Add(evt, time.Now()); shouldFlush {
			c.flush(ctx, "size")
		}
	}
}

func decodeEvent(msg kafka.Message) (entity.Event, error) {
	var wire event.RawEventPublished
	if err := json.Unmarshal(msg.Value, &wire); err != nil {
		return entity.Event{}, err
	}
	evt := entity.Event{
		EventID:    wire.EventID,
		UserID:     wire.UserID,
		EventType:  wire.EventType,
		Payload:    wire.Payload,
		OccurredAt: wire.OccurredAtValue,
	}
	return evt, evt.Validate()
}

func (c *EventConsumer) flush(ctx context.Context, trigger string) {
	events := c.batch.Drain()
	msgs := c.pending
	c.pending = nil
	if len(events) == 0 {
		return
	}

	spanCtx, span := consumerTracer.Start(ctx, "analytics.consumer.batch")
	span.SetAttributes(
		attribute.Int("batch.size", len(events)),
		attribute.String("batch.trigger", trigger),
	)
	defer span.End()

	if err := c.repo.InsertBatch(spanCtx, events); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		c.log.Error("insert batch failed, offsets not committed", zap.Error(err), zap.Int("batch_size", len(events)))
		return // do not commit — batch will be redelivered
	}

	if err := c.reader.CommitMessages(ctx, msgs...); err != nil {
		span.RecordError(err)
		c.log.Error("commit offsets after durable write", zap.Error(err))
	}
}
