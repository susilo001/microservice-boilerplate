package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	domainrepo "github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
)

var tracer = otel.Tracer("product-service/cache")

type productCache struct {
	client *redis.Client
}

func NewProductCache(client *redis.Client) domainrepo.ProductCache {
	return &productCache{client: client}
}

// Get returns nil, nil on a cache miss so callers can fall through to the DB.
func (c *productCache) Get(ctx context.Context, id string) (*entity.Product, error) {
	ctx, span := tracer.Start(ctx, "cache.product.get")
	defer span.End()
	span.SetAttributes(attribute.String("cache.key", cacheKey(id)))

	val, err := c.client.Get(ctx, cacheKey(id)).Result()
	if errors.Is(err, redis.Nil) {
		span.SetAttributes(attribute.Bool("cache.hit", false))
		return nil, nil
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("redis get product: %w", err)
	}

	span.SetAttributes(attribute.Bool("cache.hit", true))
	var product entity.Product
	if err := json.Unmarshal([]byte(val), &product); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("unmarshal cached product: %w", err)
	}
	return &product, nil
}

func (c *productCache) Set(ctx context.Context, product *entity.Product, ttl time.Duration) error {
	ctx, span := tracer.Start(ctx, "cache.product.set")
	defer span.End()
	span.SetAttributes(attribute.String("cache.key", cacheKey(product.ID)))

	data, err := json.Marshal(product)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("marshal product for cache: %w", err)
	}
	if err := c.client.Set(ctx, cacheKey(product.ID), data, ttl).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (c *productCache) Delete(ctx context.Context, id string) error {
	ctx, span := tracer.Start(ctx, "cache.product.delete")
	defer span.End()
	span.SetAttributes(attribute.String("cache.key", cacheKey(id)))

	if err := c.client.Del(ctx, cacheKey(id)).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func cacheKey(id string) string {
	return "product:" + id
}
