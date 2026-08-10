package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/usecase"
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
			seed:      nil,
			req:       request.ListProductsRequest{Page: 1, PageSize: 20},
			wantCount: 0,
			wantTotal: 0,
		},
		{
			name: "returns all products on first page",
			seed: []*entity.Product{
				{ID: "1", Name: "Alpha", Price: 1.00},
				{ID: "2", Name: "Beta", Price: 2.00},
				{ID: "3", Name: "Gamma", Price: 3.00},
			},
			req:       request.ListProductsRequest{Page: 1, PageSize: 20},
			wantCount: 3,
			wantTotal: 3,
		},
		{
			name: "respects page size",
			seed: []*entity.Product{
				{ID: "1", Name: "Alpha", Price: 1.00},
				{ID: "2", Name: "Beta", Price: 2.00},
				{ID: "3", Name: "Gamma", Price: 3.00},
			},
			req:       request.ListProductsRequest{Page: 1, PageSize: 2},
			wantCount: 2,
			wantTotal: 3,
		},
		{
			name: "filters by search",
			seed: []*entity.Product{
				{ID: "1", Name: "Alpha Widget", Price: 1.00},
				{ID: "2", Name: "Beta Gadget", Price: 2.00},
			},
			req:       request.ListProductsRequest{Page: 1, PageSize: 20, Search: "widget"},
			wantCount: 1,
			wantTotal: 1,
		},
		{
			name: "accepts whitelisted sort field",
			seed: []*entity.Product{
				{ID: "1", Name: "Alpha", Price: 1.00},
			},
			req:       request.ListProductsRequest{Page: 1, PageSize: 20, SortBy: "price", SortDir: "desc"},
			wantCount: 1,
			wantTotal: 1,
		},
		{
			name:    "rejects non-whitelisted sort field",
			seed:    []*entity.Product{{ID: "1", Name: "Alpha", Price: 1.00}},
			req:     request.ListProductsRequest{Page: 1, PageSize: 20, SortBy: "id; DROP TABLE products"},
			wantErr: entity.ErrInvalidSortField,
		},
		{
			name:    "rejects non-whitelisted preload",
			seed:    []*entity.Product{{ID: "1", Name: "Alpha", Price: 1.00}},
			req:     request.ListProductsRequest{Page: 1, PageSize: 20, Preload: []string{"NotARelation"}},
			wantErr: entity.ErrInvalidPreload,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			for _, p := range tt.seed {
				repo.products[p.ID] = p
			}
			uc := usecase.NewListProductsUseCase(repo)

			res, total, err := uc.Execute(context.Background(), tt.req)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Execute() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute() unexpected error = %v", err)
			}
			if len(res) != tt.wantCount {
				t.Errorf("got %d products, want %d", len(res), tt.wantCount)
			}
			if total != tt.wantTotal {
				t.Errorf("got total %d, want %d", total, tt.wantTotal)
			}
		})
	}
}
