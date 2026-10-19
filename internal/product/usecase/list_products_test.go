package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/usecase"
	"github.com/boilerplate/microservice-boilerplate/pkg/queryutil"
)

func TestListProductsUseCase_Execute(t *testing.T) {
	tests := []struct {
		name      string
		seed      []*entity.Product
		req       request.ListProductsRequest
		wantCount int
		wantTotal int64
		wantErr   error
	}{
		{
			name:      "returns empty slice when no products",
			req:       request.ListProductsRequest{Page: 1, PageSize: 20},
			wantCount: 0,
			wantTotal: 0,
		},
		{
			name: "filters by generic price range",
			seed: []*entity.Product{
				{ID: "1", Name: "Cheap", Price: 5},
				{ID: "2", Name: "Mid", Price: 15},
				{ID: "3", Name: "Pricey", Price: 50},
			},
			req:       request.ListProductsRequest{Page: 1, PageSize: 20, Filters: []string{"price:gte:10", "price:lte:20"}},
			wantCount: 1,
			wantTotal: 1,
		},
		{
			name: "filters by generic stock range",
			seed: []*entity.Product{
				{ID: "1", Name: "A", Stock: 0},
				{ID: "2", Name: "B", Stock: 5},
				{ID: "3", Name: "C", Stock: 100},
			},
			req:       request.ListProductsRequest{Page: 1, PageSize: 20, Filters: []string{"stock:gte:1", "stock:lte:10"}},
			wantCount: 1,
			wantTotal: 1,
		},
		{
			name:    "rejects invalid sort field",
			seed:    []*entity.Product{{ID: "1", Name: "Alpha"}},
			req:     request.ListProductsRequest{Page: 1, PageSize: 20, Sort: "id; DROP TABLE products:asc"},
			wantErr: queryutil.ErrInvalidQuery,
		},
		{
			name:    "rejects invalid filter field",
			seed:    []*entity.Product{{ID: "1", Name: "Alpha"}},
			req:     request.ListProductsRequest{Page: 1, PageSize: 20, Filters: []string{"id:eq:1"}},
			wantErr: queryutil.ErrInvalidQuery,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			for _, product := range tt.seed {
				repo.products[product.ID] = product
			}
			res, total, err := usecase.NewListProductsUseCase(repo).Execute(context.Background(), tt.req)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Execute() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute() unexpected error = %v", err)
			}
			if len(res) != tt.wantCount || total != tt.wantTotal {
				t.Errorf("got count=%d total=%d, want count=%d total=%d", len(res), total, tt.wantCount, tt.wantTotal)
			}
		})
	}
}
