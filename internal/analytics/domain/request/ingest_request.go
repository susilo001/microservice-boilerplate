package request

import "encoding/json"

// IngestEventRequest is the POST /events payload.
type IngestEventRequest struct {
	EventID   string          `json:"event_id" binding:"required"`
	UserID    string          `json:"user_id" binding:"required"`
	EventType string          `json:"event_type" binding:"required"`
	Payload   json.RawMessage `json:"payload"`
}
