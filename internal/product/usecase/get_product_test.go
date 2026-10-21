package usecase_test

import (
	"context"
	"go.uber.org/zap"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/usecase"
)

func TestGetProductUseCase_Execute(t *testing.T) {
	existing := &entity.Product{ID: "abc-123", Name: "Gadget", Price: 19.99, Stock: 5}

	tests := []struct {
		name    string
		id      string
		seed    *entity.Product
		wantErr error
	}{
		{
			name:    "returns product when found",
			id:      "abc-123",
			seed:    existing,
			wantErr: nil,
		},
		{
			name:    "returns ErrProductNotFound when missing",
			id:      "does-not-exist",
			wantErr: entity.ErrProductNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			if tt.seed != nil {
				repo.products[tt.seed.ID] = tt.seed
			}
			uc := usecase.NewGetProductUseCase(repo, &mockProductCache{}, zap.NewNop())

			res, err := uc.Execute(context.Background(), tt.id)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.ID != tt.seed.ID {
				t.Errorf("ID = %q, want %q", res.ID, tt.seed.ID)
			}
		})
	}
}
