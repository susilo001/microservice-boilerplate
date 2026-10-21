package route

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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
