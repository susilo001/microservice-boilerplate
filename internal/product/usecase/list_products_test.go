package usecase_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/usecase"
	"go.uber.org/zap"
)

func TestListProductsUseCase_Execute(t *testing.T) {
	tests := []struct {
		name      string
		seed      []*entity.Product
		req       request.ListProductsRequest
		wantCount int
		wantTotal int64
		wantErr   error
		repoErr   error
	}{
		{
			name:      "returns empty slice when no products",
			req:       request.ListProductsRequest{Page: 1, PageSize: 20},
			wantCount: 0,
			wantTotal: 0,
		},
		{
			name: "forwards structured price range and maps repository results",
			seed: []*entity.Product{
				{ID: "1", Name: "Cheap", Price: 5},
				{ID: "2", Name: "Mid", Price: 15},
				{ID: "3", Name: "Pricey", Price: 50},
			},
			req:       request.ListProductsRequest{Page: 1, PageSize: 20, Filters: []request.ProductFilter{{Field: "price", Operator: "IS_MORE_THAN_OR_EQUAL", Value: "10"}, {Field: "price", Operator: "IS_LESS_THAN_OR_EQUAL", Value: "20"}}},
			wantCount: 1,
			wantTotal: 1,
		},
		{
			name: "forwards structured stock range and maps repository results",
			seed: []*entity.Product{
				{ID: "1", Name: "A", Stock: 0},
				{ID: "2", Name: "B", Stock: 5},
				{ID: "3", Name: "C", Stock: 100},
			},
			req:       request.ListProductsRequest{Page: 1, PageSize: 20, Filters: []request.ProductFilter{{Field: "stock", Operator: "IS_MORE_THAN_OR_EQUAL", Value: "1"}, {Field: "stock", Operator: "IS_LESS_THAN_OR_EQUAL", Value: "10"}}},
			wantCount: 1,
			wantTotal: 1,
		},
		{
			name:    "rejects invalid sort field",
			seed:    []*entity.Product{{ID: "1", Name: "Alpha"}},
			req:     request.ListProductsRequest{Page: 1, PageSize: 20, Sorts: []request.ProductSort{{Field: "id; DROP TABLE products", Direction: "ASCENDING"}}},
			wantErr: entity.ErrInvalidProductQuery,
			repoErr: entity.ErrInvalidProductQuery,
		},
		{
			name:    "rejects invalid filter field",
			seed:    []*entity.Product{{ID: "1", Name: "Alpha"}},
			req:     request.ListProductsRequest{Page: 1, PageSize: 20, Filters: []request.ProductFilter{{Field: "id", Operator: "IS_EQUAL", Value: "1"}}},
			wantErr: entity.ErrInvalidProductQuery,
			repoErr: entity.ErrInvalidProductQuery,
		},
		{
			name:    "rejects excessive page size outside HTTP",
			req:     request.ListProductsRequest{PageSize: 101},
			wantErr: entity.ErrInvalidProductQuery,
		},
		{
			name:    "rejects excessive offset outside HTTP",
			req:     request.ListProductsRequest{Page: 1002, PageSize: 100},
			wantErr: entity.ErrInvalidProductQuery,
		},
		{
			name:    "rejects preloads when no relations exist",
			req:     request.ListProductsRequest{Preload: []string{"Category"}},
			wantErr: entity.ErrInvalidProductQuery,
			repoErr: entity.ErrInvalidProductQuery,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			repo.findErr = tt.repoErr
			if tt.wantCount > 0 {
				repo.listProducts = []entity.Product{*tt.seed[1]}
			}
			repo.listTotal = tt.wantTotal
			res, total, err := usecase.NewListProductsUseCase(repo, zap.NewNop()).Execute(context.Background(), tt.req)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Execute() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute() unexpected error = %v", err)
			}
			normalized, err := tt.req.Normalize()
			if err != nil || !reflect.DeepEqual(repo.listRequest, normalized) {
				t.Fatalf("repository request=%+v, want %+v; error=%v", repo.listRequest, normalized, err)
			}
			if len(res) != tt.wantCount || total != tt.wantTotal {
				t.Errorf("got count=%d total=%d, want count=%d total=%d", len(res), total, tt.wantCount, tt.wantTotal)
			}
			if tt.wantCount > 0 && (res[0].ID != tt.seed[1].ID || res[0].Name != tt.seed[1].Name || res[0].Price != tt.seed[1].Price) {
				t.Fatalf("response did not map repository result: %+v", res[0])
			}
		})
	}
}

func TestListProductsDefaultsAndRepositoryFailure(t *testing.T) {
	repo := newMockRepo()
	list := usecase.NewListProductsUseCase(repo, zap.NewNop())
	products, _, err := list.Execute(context.Background(), request.ListProductsRequest{})
	if err != nil || products == nil || repo.listRequest.Page != 1 || repo.listRequest.PageSize != 20 {
		t.Fatalf("products=%v request=%+v error=%v", products, repo.listRequest, err)
	}
	want := errors.New("database unavailable")
	repo.findErr = want
	if _, _, err := list.Execute(context.Background(), request.ListProductsRequest{}); !errors.Is(err, want) {
		t.Fatalf("error=%v, want %v", err, want)
	}
}
