package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/request"
)

type fakePublishUC struct {
	err error
}

func (f *fakePublishUC) Execute(_ context.Context, _ request.IngestEventRequest) error {
	return f.err
}

func TestIngestHandler_Ingest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		ucErr      error
		wantStatus int
	}{
		{name: "valid event returns 202", body: `{"event_id":"e1","user_id":"u1","event_type":"click"}`, wantStatus: http.StatusAccepted},
		{name: "malformed json returns 400", body: `{`, wantStatus: http.StatusBadRequest},
		{name: "missing required field returns 400", body: `{"user_id":"u1","event_type":"click"}`, wantStatus: http.StatusBadRequest},
		{name: "validation error from usecase returns 400", body: `{"event_id":"e1","user_id":"u1","event_type":"click"}`, ucErr: entity.ErrEventIDRequired, wantStatus: http.StatusBadRequest},
		{name: "publish failure returns 503", body: `{"event_id":"e1","user_id":"u1","event_type":"click"}`, ucErr: errNonValidation, wantStatus: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewIngestHandler(&fakePublishUC{err: tt.ucErr})

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewBufferString(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")

			h.Ingest(c)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

var errNonValidation = &kafkaDownError{}

type kafkaDownError struct{}

func (*kafkaDownError) Error() string { return "kafka down" }
