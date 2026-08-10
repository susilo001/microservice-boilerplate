package messaging

import (
	"time"

	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/entity"
)

// BatchAccumulator is pure logic: given events and timestamps, it decides
// when a batch should flush (size OR time, whichever comes first). It knows
// nothing about Kafka or Postgres, so it's unit-testable without either
// (AGENTS.md Good First Task #1).
type BatchAccumulator struct {
	maxSize int
	maxWait time.Duration

	events     []entity.Event
	firstAdded time.Time
}

func NewBatchAccumulator(maxSize int, maxWait time.Duration) *BatchAccumulator {
	return &BatchAccumulator{maxSize: maxSize, maxWait: maxWait}
}

// Add appends an event and reports whether the batch should flush now
// because it hit maxSize.
func (b *BatchAccumulator) Add(e entity.Event, now time.Time) (shouldFlush bool) {
	if len(b.events) == 0 {
		b.firstAdded = now
	}
	b.events = append(b.events, e)
	return len(b.events) >= b.maxSize
}

// DueByTimeout reports whether the oldest pending event has waited longer
// than maxWait. False when the batch is empty.
func (b *BatchAccumulator) DueByTimeout(now time.Time) bool {
	if len(b.events) == 0 {
		return false
	}
	return now.Sub(b.firstAdded) >= b.maxWait
}

// Drain returns the accumulated events and resets the batch.
func (b *BatchAccumulator) Drain() []entity.Event {
	events := b.events
	b.events = nil
	b.firstAdded = time.Time{}
	return events
}

func (b *BatchAccumulator) Len() int {
	return len(b.events)
}
