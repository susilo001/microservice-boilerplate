package event

import (
	"context"
	"time"
)

type Event interface {
	EventName() string
	OccurredAt() time.Time
}

type Publisher interface {
	Publish(ctx context.Context, event Event) error
}
