package usecase_test

import (
	"context"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/usecase"
)

func TestListProductsUseCase_Execute(t *testing.T) {
	tests := []struct {
		name       string
		seed       []*entity.Product
		page       int
		pageSize   int
		wantCount  int
		wantTotal  int64
		wantErr    bool
	}{
		{
			name:      "returns empty slice when no products",
			seed:      nil,
			page:      1,
			pageSize:  20,
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
			page:      1,
			pageSize:  20,
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
			page:      1,
			pageSize:  2,
			wantCount: 2,
			wantTotal: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			for _, p := range tt.seed {
				repo.products[p.ID] = p
			}
			uc := usecase.NewListProductsUseCase(repo)

			res, total, err := uc.Execute(context.Background(), tt.page, tt.pageSize)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v", err, tt.wantErr)
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
