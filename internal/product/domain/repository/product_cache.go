package repository

import (
	"context"
	"time"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
)

type ProductCache interface {
	Get(ctx context.Context, id string) (*entity.Product, error)
	Set(ctx context.Context, product *entity.Product, ttl time.Duration) error
	Delete(ctx context.Context, id string) error
}
