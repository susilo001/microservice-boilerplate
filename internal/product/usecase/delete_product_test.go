package usecase_test

import (
	"context"
	"errors"
	"go.uber.org/zap"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/usecase"
)

func TestDeleteProductUseCase_Execute(t *testing.T) {
	existing := &entity.Product{ID: "abc-123", Name: "Widget", Price: 9.99}

	tests := []struct {
		name      string
		id        string
		seed      *entity.Product
		deleteErr error
		wantErr   bool
	}{
		{
			name:    "deletes product successfully",
			id:      "abc-123",
			seed:    existing,
			wantErr: false,
		},
		{
			name:    "returns ErrProductNotFound when product does not exist",
			id:      "does-not-exist",
			wantErr: true,
		},
		{
			name:      "returns error when repo delete fails",
			id:        "abc-123",
			seed:      existing,
			deleteErr: errors.New("db error"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			if tt.seed != nil {
				repo.products[tt.seed.ID] = &entity.Product{
					ID:    tt.seed.ID,
					Name:  tt.seed.Name,
					Price: tt.seed.Price,
				}
			}
			repo.deleteErr = tt.deleteErr
			uc := usecase.NewDeleteProductUseCase(repo, &mockProductCache{}, zap.NewNop())

			err := uc.Execute(context.Background(), tt.id)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if _, ok := repo.products[tt.id]; ok {
					t.Error("expected product to be removed from repo")
				}
			}
		})
	}
}
