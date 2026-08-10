package entity

import (
	"errors"
	"time"
)

var (
	ErrEventIDRequired   = errors.New("event_id is required")
	ErrUserIDRequired    = errors.New("user_id is required")
	ErrEventTypeRequired = errors.New("event_type is required")
)

// Event is a single ingested analytics event. Payload is opaque JSON — the
// ingest path never inspects it, only the eventual consumer of the events
// table does.
type Event struct {
	EventID    string    `json:"event_id"`
	UserID     string    `json:"user_id"`
	EventType  string    `json:"event_type"`
	Payload    []byte    `json:"payload"`
	OccurredAt time.Time `json:"occurred_at"`
}

// Validate checks required fields. It does not touch the database — ingest-path
// validation must stay synchronous-DB-free (see AGENTS.md, analytics handler rule).
func (e Event) Validate() error {
	if e.EventID == "" {
		return ErrEventIDRequired
	}
	if e.UserID == "" {
		return ErrUserIDRequired
	}
	if e.EventType == "" {
		return ErrEventTypeRequired
	}
	return nil
}
