package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/event"
	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/request"
)

type fakePublisher struct {
	published event.Event
	called    bool
	err       error
}

func (f *fakePublisher) Publish(_ context.Context, evt event.Event) error {
	f.called = true
	f.published = evt
	return f.err
}

func TestPublishEventUseCase_Execute(t *testing.T) {
	validReq := request.IngestEventRequest{EventID: "evt-1", UserID: "user-1", EventType: "click"}

	tests := []struct {
		name       string
		req        request.IngestEventRequest
		publishErr error
		wantErr    bool
		wantPublish bool
	}{
		{name: "valid event publishes", req: validReq, wantPublish: true},
		{name: "missing event_id fails validation, never publishes", req: request.IngestEventRequest{UserID: "u", EventType: "t"}, wantErr: true},
		{name: "missing user_id fails validation, never publishes", req: request.IngestEventRequest{EventID: "e", EventType: "t"}, wantErr: true},
		{name: "missing event_type fails validation, never publishes", req: request.IngestEventRequest{EventID: "e", UserID: "u"}, wantErr: true},
		{name: "publish failure surfaces as error", req: validReq, publishErr: errors.New("kafka down"), wantErr: true, wantPublish: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub := &fakePublisher{err: tt.publishErr}
			uc := NewPublishEventUseCase(pub)

			err := uc.Execute(context.Background(), tt.req)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
			if pub.called != tt.wantPublish {
				t.Fatalf("publisher called = %v, want %v", pub.called, tt.wantPublish)
			}
		})
	}
}
