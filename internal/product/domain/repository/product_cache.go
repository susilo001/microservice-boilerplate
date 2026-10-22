package repository

import (
	"context"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
)

// ProductCache is optional: callers may fall back to persistence on errors.
// Operations have a 200ms budget constrained by the caller's context; the adapter
// owns transient-error retries. Successful invalidation prevents stale fills,
// but cache reads are not strongly consistent with persistence.
type ProductCache interface {
	// Get returns a product, or nil on a miss, plus a generation token for Set.
	// Obtain the token before loading from persistence. Tokens expire after 30s.
	Get(ctx context.Context, id string) (*entity.Product, string, error)
	// Set requires a nonempty token from Get for this product. Expired or rotated
	// tokens skip the fill without error; nil does not guarantee data was stored.
	// Stored data expires after 5m.
	Set(ctx context.Context, product *entity.Product, generation string) error
	// Delete atomically rotates the generation token and removes cached data.
	// Failed invalidation may leave stale data until its TTL expires.
	Delete(ctx context.Context, id string) error
}
