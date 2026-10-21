package logger

import (
	"encoding/json"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type countedPayload struct {
	calls *int
}

func (payload countedPayload) MarshalJSON() ([]byte, error) {
	*payload.calls++
	return []byte("{}"), nil
}

func TestDebugPayloadIsLazyAtInfoLevel(t *testing.T) {
	core, entries := observer.New(zap.InfoLevel)
	calls := 0
	DebugPayload(zap.New(core), "request", "request", countedPayload{calls: &calls})
	if calls != 0 || entries.Len() != 0 {
		t.Fatalf("debug data was serialized or logged at Info: calls=%d, entries=%d", calls, entries.Len())
	}
}

func TestDebugPayloadRedactsAndBoundsData(t *testing.T) {
	core, entries := observer.New(zap.DebugLevel)
	input := map[string]any{
		"name": "Widget", "price": 10,
		"nested":      []any{map[string]any{"access_token": "private-token", "PASSWORD": "private-password"}},
		"description": "private-description",
	}
	DebugPayload(zap.New(core), "request", "request", input)
	data, err := json.Marshal(entries.All()[0].ContextMap()["request"])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private-") || !strings.Contains(string(data), "Widget") ||
		!strings.Contains(string(data), "[REDACTED]") {
		t.Fatalf("unexpected sanitized payload: %s", data)
	}
	if input["description"] != "private-description" {
		t.Fatal("logging mutated the caller's payload")
	}
	DebugPayload(zap.New(core), "large", "request", strings.Repeat("private-large", 2000))
	large := entries.All()[1].ContextMap()["request"].(map[string]any)
	if large["reason"] != "payload_too_large" {
		t.Fatalf("large payload was not omitted: %v", large)
	}
}
