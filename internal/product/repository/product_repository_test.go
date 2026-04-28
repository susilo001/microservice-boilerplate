package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	domainrepo "github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
	"github.com/boilerplate/microservice-boilerplate/internal/product/repository"
	"github.com/boilerplate/microservice-boilerplate/pkg/databases"
)

// setup connects to a real Postgres instance using DB_DSN from the environment.
// The test is skipped when the variable is absent so normal CI (no DB) stays green.
func setup(t *testing.T) domainrepo.ProductRepository {
	t.Helper()
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN not set — skipping repository integration test")
	}
	db, err := databases.NewDB(databases.Config{Driver: "postgres", DSN: dsn})
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	if err := db.AutoMigrate(&entity.Product{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM products WHERE id LIKE 'test-%'") })
	return repository.NewProductRepository(db)
}

func TestProductRepository_CreateAndFindByID(t *testing.T) {
	repo := setup(t)

	p := &entity.Product{ID: "test-create-1", Name: "Widget", Price: 9.99, Stock: 5}
	if err := repo.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.FindByID(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Name != p.Name {
		t.Errorf("Name = %q, want %q", got.Name, p.Name)
	}
	if got.Price != p.Price {
		t.Errorf("Price = %v, want %v", got.Price, p.Price)
	}
}

func TestProductRepository_FindByID_NotFound(t *testing.T) {
	repo := setup(t)

	_, err := repo.FindByID(context.Background(), "test-does-not-exist")
	if !errors.Is(err, entity.ErrProductNotFound) {
		t.Errorf("expected ErrProductNotFound, got %v", err)
	}
}

func TestProductRepository_Update(t *testing.T) {
	repo := setup(t)

	p := &entity.Product{ID: "test-update-1", Name: "Old Name", Price: 1.00, Stock: 1}
	if err := repo.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	p.Name = "New Name"
	p.Price = 2.00
	if err := repo.Update(context.Background(), p); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := repo.FindByID(context.Background(), p.ID)
	if got.Name != "New Name" {
		t.Errorf("Name = %q, want %q", got.Name, "New Name")
	}
}

func TestProductRepository_Delete(t *testing.T) {
	repo := setup(t)

	p := &entity.Product{ID: "test-delete-1", Name: "Doomed", Price: 0.01, Stock: 1}
	if err := repo.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Delete(context.Background(), p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.FindByID(context.Background(), p.ID)
	if !errors.Is(err, entity.ErrProductNotFound) {
		t.Errorf("expected ErrProductNotFound after delete, got %v", err)
	}
}

func TestProductRepository_FindAllPaginated(t *testing.T) {
	repo := setup(t)

	seeds := []entity.Product{
		{ID: "test-page-1", Name: "A", Price: 1, Stock: 1},
		{ID: "test-page-2", Name: "B", Price: 2, Stock: 2},
		{ID: "test-page-3", Name: "C", Price: 3, Stock: 3},
	}
	for i := range seeds {
		if err := repo.Create(context.Background(), &seeds[i]); err != nil {
			t.Fatalf("seed Create: %v", err)
		}
	}

	products, total, err := repo.FindAllPaginated(context.Background(), 0, 2)
	if err != nil {
		t.Fatalf("FindAllPaginated: %v", err)
	}
	if total < 3 {
		t.Errorf("total = %d, want >= 3", total)
	}
	if len(products) != 2 {
		t.Errorf("page size = %d, want 2", len(products))
	}
}
