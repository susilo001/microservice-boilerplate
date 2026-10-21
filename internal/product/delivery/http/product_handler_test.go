package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/boilerplate/microservice-boilerplate/api/http/v1/route"
	producthttp "github.com/boilerplate/microservice-boilerplate/internal/product/delivery/http"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/response"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type validatingCreate struct {
	calls int
}

func (usecase *validatingCreate) Execute(ctx context.Context, req request.CreateProductRequest) (*response.ProductResponse, error) {
	usecase.calls++
	if err := entity.ValidateProduct(req.Name, req.Price, req.Stock); err != nil {
		return nil, err
	}
	return &response.ProductResponse{Name: req.Name, Price: req.Price, Stock: req.Stock}, nil
}

func TestCreateSeparatesJSONDecodingFromBusinessValidation(t *testing.T) {
	for _, test := range []struct {
		name   string
		body   string
		status int
		calls  int
	}{
		{"valid", `{"name":"Widget","price":1}`, http.StatusCreated, 1},
		{"business rejection", `{"name":"Widget","price":0}`, http.StatusBadRequest, 1},
		{"malformed", `{"name":`, http.StatusBadRequest, 0},
		{"wrong type", `{"price":"not-a-number"}`, http.StatusBadRequest, 0},
		{"trailing JSON", `{"name":"Widget","price":1} {}`, http.StatusBadRequest, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			core, logs := observer.New(zap.DebugLevel)
			log := zap.New(core)
			create := &validatingCreate{}
			handler := producthttp.NewProductHandler(nil, nil, create, nil, nil, log)
			router := route.NewRouter(handler, log, "test", route.Options{RequestTimeout: time.Second})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(test.body))
			req.Header.Set("Content-Type", "application/json")
			out := httptest.NewRecorder()
			router.ServeHTTP(out, req)
			if out.Code != test.status || create.calls != test.calls {
				t.Fatalf("status=%d calls=%d, want status=%d calls=%d", out.Code, create.calls, test.status, test.calls)
			}
			if test.calls == 0 && (logs.FilterMessage("request decoding rejected").Len() != 1 ||
				logs.FilterMessage("request decoding response").Len() != 1) {
				t.Fatal("missing decoding failure logs")
			}
		})
	}
}
