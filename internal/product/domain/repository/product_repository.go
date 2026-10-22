package repository

import (
	"context"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
)

// ProductRepository persists products with a 2s budget per operation, constrained
// by the caller's context. Callers own cache invalidation and event publishing.
type ProductRepository interface {
	// Create persists a new product and populates its timestamps on success.
	Create(ctx context.Context, product *entity.Product) error
	// FindByID reads the primary without locking and returns a detached product.
	// Missing products return entity.ErrProductNotFound.
	FindByID(ctx context.Context, id string) (*entity.Product, error)
	// FindAllPaginated validates the query and returns a bounded page plus total.
	// Reads may use replicas; the page and total are not a consistent snapshot.
	FindAllPaginated(ctx context.Context, params request.ListProductsRequest) ([]entity.Product, int64, error)
	// Update writes all editable fields, including zero values, using ID and Version.
	// It never inserts: missing products return entity.ErrProductNotFound and stale
	// versions return entity.ErrProductConflict. Success increments product.Version
	// and populates product.UpdatedAt; failure leaves the supplied product unchanged.
	Update(ctx context.Context, product *entity.Product) error
	// Delete removes the product or returns entity.ErrProductNotFound.
	Delete(ctx context.Context, id string) error
}
