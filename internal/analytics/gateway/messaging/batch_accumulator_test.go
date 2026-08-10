package messaging

import (
	"testing"
	"time"

	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/entity"
)

func TestBatchAccumulator_SizeTrigger(t *testing.T) {
	b := NewBatchAccumulator(3, time.Hour)
	now := time.Now()

	if flush := b.Add(entity.Event{EventID: "1"}, now); flush {
		t.Fatalf("expected no flush after 1 event")
	}
	if flush := b.Add(entity.Event{EventID: "2"}, now); flush {
		t.Fatalf("expected no flush after 2 events")
	}
	flush := b.Add(entity.Event{EventID: "3"}, now)
	if !flush {
		t.Fatalf("expected flush at maxSize")
	}

	events := b.Drain()
	if len(events) != 3 {
		t.Fatalf("expected 3 drained events, got %d", len(events))
	}
	if b.Len() != 0 {
		t.Fatalf("expected empty batch after drain, got %d", b.Len())
	}
}

func TestBatchAccumulator_TimeoutTrigger(t *testing.T) {
	b := NewBatchAccumulator(100, 200*time.Millisecond)
	start := time.Now()

	if due := b.DueByTimeout(start); due {
		t.Fatalf("expected not due on empty batch")
	}

	b.Add(entity.Event{EventID: "1"}, start)

	if due := b.DueByTimeout(start.Add(100 * time.Millisecond)); due {
		t.Fatalf("expected not due before maxWait elapses")
	}
	if due := b.DueByTimeout(start.Add(200 * time.Millisecond)); !due {
		t.Fatalf("expected due once maxWait elapses")
	}
}

func TestBatchAccumulator_DrainResets(t *testing.T) {
	b := NewBatchAccumulator(2, time.Hour)
	now := time.Now()
	b.Add(entity.Event{EventID: "1"}, now)

	_ = b.Drain()

	if due := b.DueByTimeout(now.Add(time.Hour)); due {
		t.Fatalf("expected DueByTimeout false after drain (empty batch)")
	}
	if b.Len() != 0 {
		t.Fatalf("expected len 0 after drain")
	}
}
