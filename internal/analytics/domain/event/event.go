package event

import (
	"context"
	"time"
)

// Event mirrors product/domain/event.Event. Domains don't import each other's
// packages (AGENTS.md dependency rule), so this small interface is duplicated
// rather than shared.
type Event interface {
	EventName() string
	OccurredAt() time.Time
}

type Publisher interface {
	Publish(ctx context.Context, event Event) error
}
