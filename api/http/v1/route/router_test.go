package route

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestHTTPFailureContracts(t *testing.T) {
	router := NewRouter(nil, zap.NewNop(), "test", Options{
		RequestTimeout: time.Second,
		Ready:          func(context.Context) error { return errors.New("private database detail") },
	})
	router.GET("/failure", func(ctx *gin.Context) {
		_ = ctx.Error(errors.New("private database detail"))
	})
	router.GET("/panic", func(ctx *gin.Context) {
		panic("private panic detail")
	})
	router.GET("/deadline", func(ctx *gin.Context) {
		if _, ok := ctx.Request.Context().Deadline(); !ok {
			t.Error("request is missing deadline")
		}
		_ = ctx.Error(context.DeadlineExceeded)
	})
	for _, test := range []struct {
		path   string
		status int
	}{
		{"/health", http.StatusOK},
		{"/ready", http.StatusServiceUnavailable},
		{"/failure", http.StatusInternalServerError},
		{"/panic", http.StatusInternalServerError},
		{"/deadline", http.StatusGatewayTimeout},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			if response.Code != test.status || strings.Contains(response.Body.String(), "private") {
				t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestPanicLoggingUsesStructuredOutputWithoutRequestHeaders(t *testing.T) {
	var frameworkOutput bytes.Buffer
	previousWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &frameworkOutput
	t.Cleanup(func() { gin.DefaultErrorWriter = previousWriter })
	core, entries := observer.New(zap.DebugLevel)
	router := NewRouter(nil, zap.New(core), "test", Options{RequestTimeout: time.Second})
	router.GET("/panic-log", func(ctx *gin.Context) { panic("test panic") })
	request := httptest.NewRequest(http.MethodGet, "/panic-log", nil)
	request.Header.Set("Cookie", "session=secret-value")
	request.Header.Set("X-Correlation-ID", "panic-request")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError || frameworkOutput.Len() != 0 {
		t.Fatalf("unexpected panic response or framework output: %d %s", response.Code, frameworkOutput.String())
	}
	panics := entries.FilterMessage("request panic").All()
	if len(panics) != 1 || panics[0].Level != zap.ErrorLevel {
		t.Fatalf("expected one structured panic diagnostic: %+v", panics)
	}
	fields := panics[0].ContextMap()
	if fields["correlationid"] != "panic-request" || fields["panic_stack"] == "" {
		t.Fatalf("missing panic context: %v", fields)
	}
	for _, entry := range entries.All() {
		for _, field := range entry.Context {
			if strings.Contains(field.String, "secret-value") {
				t.Fatal("request header leaked into logs")
			}
		}
	}
}
