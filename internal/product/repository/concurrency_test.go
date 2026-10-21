package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/repository"
	"github.com/boilerplate/microservice-boilerplate/pkg/databases"
)

func TestUpdateRejectsConcurrentChangesAndDeletedProducts(t *testing.T) {
	db, err := databases.NewDB(databases.Config{Driver: "sqlite", DSN: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { pool.Close() })
	if err := db.AutoMigrate(&entity.Product{}); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewProductRepository(db)
	ctx := context.Background()
	product := &entity.Product{ID: "concurrency-test", Name: "Original", Price: 1, Stock: 1}
	if err := repo.Create(ctx, product); err != nil {
		t.Fatal(err)
	}
	first, err := repo.FindByIDForUpdate(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.FindByIDForUpdate(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	first.Name = "Winner"
	first.Stock = 0
	if err := repo.Update(ctx, first); err != nil {
		t.Fatal(err)
	}
	second.Name = "Stale"
	if err := repo.Update(ctx, second); !errors.Is(err, entity.ErrProductConflict) {
		t.Fatalf("stale update error = %v, want conflict", err)
	}
	persisted, err := repo.FindByID(ctx, product.ID)
	if err != nil || persisted.Name != "Winner" || persisted.Stock != 0 || persisted.Version != 1 {
		t.Fatalf("unexpected stored product: %+v, error=%v", persisted, err)
	}
	if err := repo.Delete(ctx, product.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, first); !errors.Is(err, entity.ErrProductNotFound) {
		t.Fatalf("update deleted product error = %v", err)
	}
	if _, err := repo.FindByID(ctx, product.ID); !errors.Is(err, entity.ErrProductNotFound) {
		t.Fatalf("deleted product was recreated: %v", err)
	}
}
