package repository

import (
	"context"

	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/entity"
)

// EventRepository is implemented by internal/analytics/repository using
// pgxpool + COPY (see AGENTS.md "GORM exception"). Never implemented with GORM.
type EventRepository interface {
	// InsertBatch bulk-loads events and is idempotent on (event_id, occurred_at).
	InsertBatch(ctx context.Context, events []entity.Event) error
}
