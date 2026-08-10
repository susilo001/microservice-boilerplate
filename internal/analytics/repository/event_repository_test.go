package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/entity"
)

// TestEventRepository_InsertBatch runs against a real Postgres instance.
// Skipped unless ANALYTICS_DB_DSN is set (AGENTS.md testing convention).
func TestEventRepository_InsertBatch(t *testing.T) {
	dsn := os.Getenv("ANALYTICS_DB_DSN")
	if dsn == "" {
		t.Skip("ANALYTICS_DB_DSN not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	repo := NewEventRepository(pool)

	now := time.Now().UTC()
	events := []entity.Event{
		{EventID: "test-evt-1", UserID: "user-1", EventType: "click", Payload: []byte(`{}`), OccurredAt: now},
		{EventID: "test-evt-2", UserID: "user-2", EventType: "view", Payload: []byte(`{}`), OccurredAt: now},
	}

	if err := repo.InsertBatch(ctx, events); err != nil {
		t.Fatalf("InsertBatch() error = %v", err)
	}

	// idempotency: re-inserting the same batch must not error (ON CONFLICT DO NOTHING)
	if err := repo.InsertBatch(ctx, events); err != nil {
		t.Fatalf("InsertBatch() second call (dedup) error = %v", err)
	}

	if err := repo.InsertBatch(ctx, nil); err != nil {
		t.Fatalf("InsertBatch(nil) error = %v", err)
	}
}
