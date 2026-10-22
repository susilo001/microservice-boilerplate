package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/event"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/usecase"
	"go.uber.org/zap"
)

func TestUpdateProductUseCase_Execute(t *testing.T) {
	existing := &entity.Product{ID: "abc-123", Name: "Old Name", Price: 9.99, Stock: 5, Version: 4}

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
		{
			name:      "returns conflict without publishing",
			id:        "abc-123",
			req:       request.UpdateProductRequest{Name: "New Name", Price: 19.99},
			seed:      existing,
			updateErr: entity.ErrProductConflict,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			if tt.seed != nil {
				seed := *tt.seed
				repo.products[seed.ID] = &seed
			}
			repo.updateErr = tt.updateErr
			publisher := &mockPublisher{}
			uc := usecase.NewUpdateProductUseCase(repo, &mockProductCache{}, publisher, zap.NewNop())

			res, err := uc.Execute(context.Background(), tt.id, tt.req)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if res != nil || len(publisher.published) != 0 {
					t.Fatal("failed update returned a response or published an event")
				}
				if tt.updateErr != nil && !errors.Is(err, tt.updateErr) {
					t.Fatalf("error = %v, want %v", err, tt.updateErr)
				}
				if tt.seed != nil {
					stored, findErr := repo.FindByID(context.Background(), tt.id)
					if findErr != nil || *stored != *tt.seed {
						t.Fatalf("failed update changed stored product: product=%+v, error=%v", stored, findErr)
					}
				}
				return
			}
			if res.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", res.Name, tt.wantName)
			}
			if res.Price != tt.wantPrice {
				t.Errorf("Price = %v, want %v", res.Price, tt.wantPrice)
			}
			if len(publisher.published) != 1 {
				t.Fatalf("expected 1 event published, got %d", len(publisher.published))
			}
			stored, err := repo.FindByID(context.Background(), tt.id)
			if err != nil {
				t.Fatal(err)
			}
			published, ok := publisher.published[0].(event.ProductUpdatedEvent)
			if !ok || published.Version != tt.seed.Version+1 || published.Version != stored.Version {
				t.Fatalf("published event = %+v, stored version = %d", publisher.published[0], stored.Version)
			}
			if !published.UpdatedAt.Equal(stored.UpdatedAt) || !res.UpdatedAt.Equal(stored.UpdatedAt) {
				t.Fatal("response and event timestamps must match the committed product")
			}
		})
	}
}
