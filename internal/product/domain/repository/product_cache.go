package repository

import (
	"context"
	"time"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
)

type ProductCache interface {
	Get(ctx context.Context, id string) (*entity.Product, string, error)
	Set(ctx context.Context, product *entity.Product, ttl time.Duration, generation string) error
	Delete(ctx context.Context, id string) error
}
