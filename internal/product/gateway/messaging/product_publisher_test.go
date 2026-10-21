package messaging

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/event"
	"github.com/segmentio/kafka-go"
)

type recordingWriter struct {
	messages []kafka.Message
	attempts int
	err      error
}

func (writer *recordingWriter) WriteMessages(ctx context.Context, messages ...kafka.Message) error {
	writer.attempts++
	writer.messages = append(writer.messages, messages...)
	if writer.attempts == 1 {
		return writer.err
	}
	return nil
}

func TestPublisherRetriesWithStableIdentityAndKey(t *testing.T) {
	writer := &recordingWriter{err: kafka.LeaderNotAvailable}
	publisher := NewProductPublisher(writer)
	productEvent := event.ProductCreatedEvent{EventID: "event-1", SchemaVersion: 1, ProductID: "product-1"}
	if err := publisher.Publish(context.Background(), productEvent); err != nil {
		t.Fatal(err)
	}
	if writer.attempts != 2 {
		t.Fatalf("attempts = %d, want 2", writer.attempts)
	}
	for _, message := range writer.messages {
		var decoded event.ProductCreatedEvent
		if err := json.Unmarshal(message.Value, &decoded); err != nil {
			t.Fatal(err)
		}
		if string(message.Key) != productEvent.ProductID || decoded.EventID != productEvent.EventID ||
			decoded.SchemaVersion != 1 {
			t.Fatalf("event identity or key changed: %+v", message)
		}
	}
}

func TestPublisherDoesNotRetryPermanentErrors(t *testing.T) {
	writer := &recordingWriter{err: kafka.WriteErrors{kafka.MessageSizeTooLarge}}
	if err := NewProductPublisher(writer).Publish(context.Background(), event.ProductCreatedEvent{}); err == nil {
		t.Fatal("expected publish failure")
	}
	if writer.attempts != 1 {
		t.Fatalf("attempts = %d, want 1", writer.attempts)
	}
}
