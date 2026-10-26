package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/api/http/v1/route"
	producthttp "github.com/boilerplate/microservice-boilerplate/internal/product/delivery/http"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	domainrepo "github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/response"
	"github.com/boilerplate/microservice-boilerplate/internal/product/usecase"
	"go.uber.org/zap"
)

type capturingList struct {
	params request.ListProductsRequest
	calls  int
}

func (list *capturingList) Execute(_ context.Context, params request.ListProductsRequest) ([]response.ProductResponse, int64, error) {
	list.params = params
	list.calls++
	return []response.ProductResponse{}, 0, nil
}

func TestProductListParsesStructuredQuery(t *testing.T) {
	list := &capturingList{}
	handler := producthttp.NewProductHandler(nil, list, nil, nil, nil, zap.NewNop())
	router := route.NewRouter(handler, zap.NewNop(), "test", route.Options{})
	values := url.Values{
		"filter": {"price:10:greaterthanorequal|stock:0:equals"},
		"sort":   {"price:desc,name:asc"}, "search": {"widget"}, "page": {"2"}, "page_size": {"5"},
	}
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/api/v1/products?"+values.Encode(), nil))
	if out.Code != http.StatusOK || list.calls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", out.Code, list.calls, out.Body.String())
	}
	params := list.params
	if params.Page != 2 || params.PageSize != 5 || params.Search != "widget" || len(params.Filters) != 2 || len(params.Sorts) != 2 {
		t.Fatalf("unexpected parsed request: %+v", params)
	}
	if params.Filters[0] != (request.ProductFilter{Field: "price", Operator: "IS_MORE_THAN_OR_EQUAL", Value: "10"}) || params.Sorts[0] != (request.ProductSort{Field: "price", Direction: "DESCENDING"}) {
		t.Fatalf("unexpected structured query: %+v", params)
	}
}

func TestProductListRejectsMalformedTransportQueries(t *testing.T) {
	for _, query := range []string{
		"page=0", "page=-1", "page=", "page=oops", "page_size=0", "page_size=", "page_size=101",
		"page=1002&page_size=100", "filter=price:gte:10", "filter=price:10", "sort=price:sideways", "fields=name", "fields=",
		"filter=", "filter=%7C", "filter=price:10:equals%7C", "filter=price:10:equals%7C%7Cstock:0:equals",
		"filter=price:10:equals&filter=", "sort=", "sort=%2C", "sort=price:asc%2C", "sort=price:asc&sort=name:desc",
		"preload=", "preload=Category%2C", "filter=price:10:private-error-detail",
	} {
		t.Run(query, func(t *testing.T) {
			list := &capturingList{}
			handler := producthttp.NewProductHandler(nil, list, nil, nil, nil, zap.NewNop())
			router := route.NewRouter(handler, zap.NewNop(), "test", route.Options{})
			out := httptest.NewRecorder()
			router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/api/v1/products?"+query, nil))
			if out.Code != http.StatusBadRequest || list.calls != 0 {
				t.Fatalf("status=%d calls=%d", out.Code, list.calls)
			}
			if strings.Contains(out.Body.String(), "private-error-detail") {
				t.Fatalf("internal parser detail leaked in response: %s", out.Body.String())
			}
		})
	}
}

func TestProductListAcceptsRepeatedFilters(t *testing.T) {
	list := &capturingList{}
	handler := producthttp.NewProductHandler(nil, list, nil, nil, nil, zap.NewNop())
	router := route.NewRouter(handler, zap.NewNop(), "test", route.Options{})
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/api/v1/products?filter=price:10:greaterthanorequal&filter=stock:0:equals", nil))
	if out.Code != http.StatusOK || list.calls != 1 || len(list.params.Filters) != 2 {
		t.Fatalf("status=%d calls=%d filters=%v", out.Code, list.calls, list.params.Filters)
	}
}

type rejectingListRepository struct{ domainrepo.ProductRepository }

func (repo *rejectingListRepository) FindAllPaginated(context.Context, request.ListProductsRequest) ([]entity.Product, int64, error) {
	return nil, 0, entity.ErrInvalidProductQuery
}

func TestProductListMapsRepositoryRejectionTo400(t *testing.T) {
	list := usecase.NewListProductsUseCase(&rejectingListRepository{}, zap.NewNop())
	handler := producthttp.NewProductHandler(nil, list, nil, nil, nil, zap.NewNop())
	router := route.NewRouter(handler, zap.NewNop(), "test", route.Options{})
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/api/v1/products?sort=unknown:asc", nil))
	if out.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", out.Code, out.Body.String())
	}
}

func TestProductListDefaultsReachResponseMetadata(t *testing.T) {
	list := &capturingList{}
	handler := producthttp.NewProductHandler(nil, list, nil, nil, nil, zap.NewNop())
	router := route.NewRouter(handler, zap.NewNop(), "test", route.Options{})
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/api/v1/products", nil))
	if out.Code != http.StatusOK || list.params.Page != 1 || list.params.PageSize != 20 || !strings.Contains(out.Body.String(), `"page_size":20`) {
		t.Fatalf("params=%+v status=%d body=%s", list.params, out.Code, out.Body.String())
	}
}
