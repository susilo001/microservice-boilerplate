package usecase_test

import (
	"context"
	"errors"
	"go.uber.org/zap"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/usecase"
)

func TestUpdateProductUseCase_Execute(t *testing.T) {
	existing := &entity.Product{ID: "abc-123", Name: "Old Name", Price: 9.99, Stock: 5}

	tests := []struct {
		name      string
		id        string
		req       request.UpdateProductRequest
		seed      *entity.Product
		findErr   error
		updateErr error
		wantErr   bool
		wantName  string
		wantPrice float64
	}{
		{
			name:      "updates product successfully",
			id:        "abc-123",
			req:       request.UpdateProductRequest{Name: "New Name", Price: 19.99, Stock: 10},
			seed:      existing,
			wantName:  "New Name",
			wantPrice: 19.99,
		},
		{
			name:    "returns error when product not found",
			id:      "does-not-exist",
			req:     request.UpdateProductRequest{Name: "New Name", Price: 19.99},
			wantErr: true,
		},
		{
			name:      "returns error when repo update fails",
			id:        "abc-123",
			req:       request.UpdateProductRequest{Name: "New Name", Price: 19.99},
			seed:      existing,
			updateErr: errors.New("db error"),
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
					Stock: tt.seed.Stock,
				}
			}
			repo.updateErr = tt.updateErr
			publisher := &mockPublisher{}
			uc := usecase.NewUpdateProductUseCase(repo, &mockProductCache{}, publisher, zap.NewNop())

			res, err := uc.Execute(context.Background(), tt.id, tt.req)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if res.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", res.Name, tt.wantName)
			}
			if res.Price != tt.wantPrice {
				t.Errorf("Price = %v, want %v", res.Price, tt.wantPrice)
			}
			if len(publisher.published) != 1 {
				t.Errorf("expected 1 event published, got %d", len(publisher.published))
			}
		})
	}
}
