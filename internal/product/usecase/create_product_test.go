package usecase_test

import (
	"context"
	"errors"
	"go.uber.org/zap"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/usecase"
)

func TestCreateProductUseCase_Execute(t *testing.T) {
	tests := []struct {
		name      string
		req       request.CreateProductRequest
		repoErr   error
		wantErr   bool
		wantName  string
		wantPrice float64
	}{
		{
			name:      "creates product successfully",
			req:       request.CreateProductRequest{Name: "Widget", Price: 9.99, Stock: 10},
			wantName:  "Widget",
			wantPrice: 9.99,
		},
		{
			name:    "returns error when repo fails",
			req:     request.CreateProductRequest{Name: "Widget", Price: 9.99},
			repoErr: errors.New("db connection lost"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			repo.createErr = tt.repoErr
			publisher := &mockPublisher{}
			uc := usecase.NewCreateProductUseCase(repo, publisher, zap.NewNop())

			res, err := uc.Execute(context.Background(), tt.req)

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
			if res.ID == "" {
				t.Error("expected non-empty ID")
			}
			if len(publisher.published) != 1 {
				t.Errorf("expected 1 event published, got %d", len(publisher.published))
			}
		})
	}
}
