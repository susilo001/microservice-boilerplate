package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/entity"
)

// eventRepository is the documented GORM exception (AGENTS.md): pgxpool +
// staging-table + COPY + INSERT ... ON CONFLICT DO NOTHING. Do not rewrite
// this to use GORM's CreateInBatches — that reintroduces the row-oriented
// insert bottleneck this domain exists to avoid.
type eventRepository struct {
	pool   *pgxpool.Pool
	tracer trace.Tracer
}

func NewEventRepository(pool *pgxpool.Pool) *eventRepository {
	return &eventRepository{pool: pool, tracer: otel.Tracer("analytics/repository")}
}

// InsertBatch loads events into a per-transaction temp staging table via the
// COPY protocol, then moves them into events with ON CONFLICT DO NOTHING so
// Kafka's at-least-once delivery can't produce duplicate rows.
func (r *eventRepository) InsertBatch(ctx context.Context, events []entity.Event) error {
	ctx, span := r.tracer.Start(ctx, "analytics.repository.InsertBatch")
	defer span.End()

	if len(events) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE staged_events (
			event_id    TEXT NOT NULL,
			user_id     TEXT NOT NULL,
			event_type  TEXT NOT NULL,
			payload     JSONB,
			occurred_at TIMESTAMPTZ NOT NULL
		) ON COMMIT DROP
	`); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create staging table: %w", err)
	}

	rows := make([][]any, len(events))
	for i, e := range events {
		rows[i] = []any{e.EventID, e.UserID, e.EventType, e.Payload, e.OccurredAt}
	}

	if _, err := tx.CopyFrom(ctx,
		pgx.Identifier{"staged_events"},
		[]string{"event_id", "user_id", "event_type", "payload", "occurred_at"},
		pgx.CopyFromRows(rows),
	); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("copy staged_events: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO events (event_id, user_id, event_type, payload, occurred_at)
		SELECT event_id, user_id, event_type, payload, occurred_at FROM staged_events
		ON CONFLICT (event_id, occurred_at) DO NOTHING
	`); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("insert from staging: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
