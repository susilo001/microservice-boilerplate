package repository

import (
	"context"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
)

// ListParams carries already-validated pagination/search/sort/preload
// options for FindAllPaginated. SortBy and Preload must be whitelisted by
// the caller (see internal/product/usecase's allowedSortFields /
// allowedPreloads) before reaching this struct — the repository trusts
// them and interpolates them directly into ORDER BY / Preload() calls.
type ListParams struct {
	Offset  int
	Limit   int
	Search  string
	SortBy  string
	SortDir string
	Preload []string
}

type ProductRepository interface {
	Create(ctx context.Context, product *entity.Product) error
	FindByID(ctx context.Context, id string) (*entity.Product, error)
	FindAll(ctx context.Context) ([]entity.Product, error)
	FindAllPaginated(ctx context.Context, params ListParams) ([]entity.Product, int64, error)
	Update(ctx context.Context, product *entity.Product) error
	Delete(ctx context.Context, id string) error
}
