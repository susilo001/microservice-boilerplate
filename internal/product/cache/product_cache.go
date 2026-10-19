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
	"go.opentelemetry.io/otel/metric"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	domainrepo "github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
	"github.com/boilerplate/microservice-boilerplate/pkg/retry"
)

var tracer = otel.Tracer("product-service/cache")

type productCache struct {
	client       *redis.Client
	errorCounter metric.Int64Counter
}

func NewProductCache(client *redis.Client) domainrepo.ProductCache {
	errorCounter, _ := otel.Meter("product-service/cache").Int64Counter(
		"product.cache.errors",
		metric.WithDescription("Number of product cache operation failures"),
	)
	return &productCache{client: client, errorCounter: errorCounter}
}

// Get returns nil, nil on a cache miss so callers can fall through to the DB.
func (c *productCache) Get(ctx context.Context, id string) (*entity.Product, error) {
	ctx, span := tracer.Start(ctx, "cache.product.get")
	defer span.End()
	span.SetAttributes(attribute.String("cache.key", cacheKey(id)))

	var val string
	cacheMiss := false
	err := retry.Do(ctx, retry.DefaultConfig(), func() error {
		var getErr error
		val, getErr = c.client.Get(ctx, cacheKey(id)).Result()
		cacheMiss = errors.Is(getErr, redis.Nil)
		if cacheMiss {
			return nil
		}
		return getErr
	})
	if cacheMiss {
		span.SetAttributes(attribute.Bool("cache.hit", false))
		return nil, nil
	}
	if err != nil {
		c.recordError(ctx, "get")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("redis get product: %w", err)
	}

	span.SetAttributes(attribute.Bool("cache.hit", true))
	var product entity.Product
	if err := json.Unmarshal([]byte(val), &product); err != nil {
		c.recordError(ctx, "get")
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
		c.recordError(ctx, "set")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("marshal product for cache: %w", err)
	}
	if err := retry.Do(ctx, retry.DefaultConfig(), func() error {
		return c.client.Set(ctx, cacheKey(product.ID), data, ttl).Err()
	}); err != nil {
		c.recordError(ctx, "set")
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

	if err := retry.Do(ctx, retry.DefaultConfig(), func() error {
		return c.client.Del(ctx, cacheKey(id)).Err()
	}); err != nil {
		c.recordError(ctx, "delete")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (c *productCache) recordError(ctx context.Context, operation string) {
	c.errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("cache.operation", operation)))
}

func cacheKey(id string) string {
	return "product:" + id
}
