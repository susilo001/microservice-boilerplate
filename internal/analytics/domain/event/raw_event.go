package event

import "time"

// RawEventPublished is the Kafka wire event published for every ingested
// analytics event. Topic name lives in config (KAFKA_TOPIC), not here.
type RawEventPublished struct {
	EventID         string    `json:"event_id"`
	UserID          string    `json:"user_id"`
	EventType       string    `json:"event_type"`
	Payload         []byte    `json:"payload"`
	OccurredAtValue time.Time `json:"occurred_at"`
}

func (e RawEventPublished) EventName() string    { return "raw-events" }
func (e RawEventPublished) OccurredAt() time.Time { return e.OccurredAtValue }
