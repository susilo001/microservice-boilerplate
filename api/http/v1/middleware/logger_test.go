package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestSeverity(t *testing.T) {
	for _, test := range []struct {
		name   string
		path   string
		status int
		level  zapcore.Level
	}{
		{"success", "/test", http.StatusOK, zap.InfoLevel},
		{"client error", "/test", http.StatusBadRequest, zap.WarnLevel},
		{"missing", "/test", http.StatusNotFound, zap.WarnLevel},
		{"server error", "/test", http.StatusInternalServerError, zap.ErrorLevel},
		{"liveness", "/health", http.StatusOK, zap.DebugLevel},
		{"readiness", "/ready", http.StatusOK, zap.DebugLevel},
		{"failed readiness", "/ready", http.StatusServiceUnavailable, zap.ErrorLevel},
	} {
		t.Run(test.name, func(t *testing.T) {
			core, entries := observer.New(zap.DebugLevel)
			router := gin.New()
			router.Use(CorrelationID(), Logger(zap.New(core)))
			router.GET(test.path, func(ctx *gin.Context) { ctx.Status(test.status) })
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			request.Header.Set("X-Correlation-ID", "test-request")
			router.ServeHTTP(httptest.NewRecorder(), request)
			if entries.Len() != 1 {
				t.Fatalf("got %d request logs, want 1", entries.Len())
			}
			entry := entries.All()[0]
			if entry.Level != test.level || entry.ContextMap()["correlationid"] != "test-request" {
				t.Fatalf("unexpected request log: %+v", entry)
			}
		})
	}
}
