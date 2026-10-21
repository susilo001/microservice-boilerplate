package repository_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/repository"
	"github.com/boilerplate/microservice-boilerplate/pkg/databases"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

func TestProductRepositoryReadRouting(t *testing.T) {
	for _, name := range []string{"primary only", "with lagging replica"} {
		t.Run(name, func(t *testing.T) {
			openDatabase := func(path string) *gorm.DB {
				t.Helper()
				database, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := databases.Close(database); err != nil {
						t.Error(err)
					}
				})
				if err := database.AutoMigrate(&entity.Product{}); err != nil {
					t.Fatal(err)
				}
				return database
			}
			primary := openDatabase(filepath.Join(t.TempDir(), "primary.db"))
			if err := primary.Create(&entity.Product{ID: "shared", Name: "Fresh", Price: 1, Version: 2}).Error; err != nil {
				t.Fatal(err)
			}
			if err := primary.Create(&entity.Product{ID: "primary-only", Name: "New", Price: 1}).Error; err != nil {
				t.Fatal(err)
			}
			if name == "with lagging replica" {
				replicaPath := filepath.Join(t.TempDir(), "replica.db")
				replica := openDatabase(replicaPath)
				if err := replica.Create(&entity.Product{ID: "shared", Name: "Stale", Price: 1, Version: 1}).Error; err != nil {
					t.Fatal(err)
				}
				if err := primary.Use(dbresolver.Register(dbresolver.Config{
					Replicas: []gorm.Dialector{sqlite.Open(replicaPath)},
				})); err != nil {
					t.Fatal(err)
				}
			}
			repo := repository.NewProductRepository(primary)
			ctx := context.Background()
			product, err := repo.FindByID(ctx, "shared")
			if err != nil {
				t.Fatal(err)
			}
			if product.Name != "Fresh" || product.Version != 2 {
				t.Fatalf("single-product read used stale data: %+v", product)
			}
			if _, err := repo.FindByID(ctx, "primary-only"); err != nil {
				t.Fatalf("new primary record was not readable: %v", err)
			}
			if err := product.ChangeDetails(entity.ProductDetails{Name: "Updated", Price: 2}); err != nil {
				t.Fatal(err)
			}
			if err := repo.Update(ctx, product); err != nil {
				t.Fatalf("update should use current primary version: %v", err)
			}
			updated, err := repo.FindByID(ctx, "shared")
			if err != nil || updated.Name != "Updated" || updated.Version != 3 {
				t.Fatalf("read after update = %+v, error = %v", updated, err)
			}
			products, total, err := repo.FindAllPaginated(ctx, request.ListProductsRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if name == "with lagging replica" && (total != 1 || len(products) != 1 || products[0].Name != "Stale") {
				t.Fatalf("list should retain replica routing: total=%d products=%+v", total, products)
			}
			if name == "primary only" && (total != 2 || len(products) != 2) {
				t.Fatalf("list should work without replicas: total=%d products=%+v", total, products)
			}
		})
	}
}
