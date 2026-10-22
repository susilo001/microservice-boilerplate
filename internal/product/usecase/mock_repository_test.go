package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
)

func TestMockRepositoryDetachesStoredProducts(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	product := &entity.Product{ID: "product-1", Name: "Original", Price: 10, Stock: 5}
	if err := repo.Create(ctx, product); err != nil {
		t.Fatal(err)
	}
	product.Name = "Uncommitted create mutation"
	loaded, err := repo.FindByID(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "Original" {
		t.Fatal("Create retained the caller's product pointer")
	}
	loaded.Name = "Updated"
	before, err := repo.FindByID(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Name != "Original" {
		t.Fatal("FindByID exposed the stored product pointer")
	}
	loaded.Description = ""
	loaded.Stock = 0
	if err := repo.Update(ctx, loaded); err != nil {
		t.Fatal(err)
	}
	committed := *loaded
	loaded.Name = "Uncommitted update mutation"
	stored, err := repo.FindByID(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if *stored != committed || stored.Version != before.Version+1 || stored.Stock != 0 {
		t.Fatalf("stored product = %+v, want committed product %+v with incremented version and zero stock", stored, committed)
	}
	if stored.CreatedAt != before.CreatedAt || stored.UpdatedAt.IsZero() {
		t.Fatal("Update must preserve creation time and populate update time")
	}
}

func TestMockRepositoryRejectsStaleOrDeletedProduct(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		name := "stale version"
		if deleted {
			name = "deleted product"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			repo := newMockRepo()
			product := &entity.Product{ID: "product-1", Name: "Original", Price: 10, Version: 4}
			if err := repo.Create(ctx, product); err != nil {
				t.Fatal(err)
			}
			stale, err := repo.FindByID(ctx, product.ID)
			if err != nil {
				t.Fatal(err)
			}
			wantErr := entity.ErrProductConflict
			if deleted {
				if err := repo.Delete(ctx, product.ID); err != nil {
					t.Fatal(err)
				}
				wantErr = entity.ErrProductNotFound
			} else {
				product.Name = "Winner"
				if err := repo.Update(ctx, product); err != nil {
					t.Fatal(err)
				}
			}
			stale.Name = "Loser"
			before := *stale
			if err := repo.Update(ctx, stale); !errors.Is(err, wantErr) {
				t.Fatalf("Update error = %v, want %v", err, wantErr)
			}
			if *stale != before {
				t.Fatal("rejected update changed the supplied product")
			}
			stored, err := repo.FindByID(ctx, product.ID)
			if deleted {
				if !errors.Is(err, entity.ErrProductNotFound) {
					t.Fatalf("deleted product was recreated: product=%+v, error=%v", stored, err)
				}
			} else if err != nil || *stored != *product {
				t.Fatalf("conflict changed committed product: product=%+v, error=%v", stored, err)
			}
		})
	}
}
