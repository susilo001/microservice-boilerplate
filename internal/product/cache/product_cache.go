package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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

// Get returns a nil product and generation token on a cache miss.
func (c *productCache) Get(ctx context.Context, id string) (*entity.Product, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	ctx, span := tracer.Start(ctx, "cache.product.get")
	defer span.End()
	span.SetAttributes(attribute.String("cache.key", cacheKey(id)))

	var values []interface{}
	token := uuid.NewString()
	err := retry.Do(ctx, cacheRetryPolicy(), func() error {
		var getErr error
		values, getErr = c.client.Eval(ctx, `
local token = redis.call('GET', KEYS[2])
if not token then
    token = ARGV[1]
    redis.call('SET', KEYS[2], token, 'PX', 30000)
end
return {redis.call('GET', KEYS[1]) or '', token}
`, []string{cacheKey(id), generationKey(id)}, token).Slice()
		return getErr
	})
	if err != nil {
		c.recordError(ctx, "get")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, "", fmt.Errorf("redis get product: %w", err)
	}
	val := values[0].(string)
	generation := values[1].(string)
	if val == "" {
		span.SetAttributes(attribute.Bool("cache.hit", false))
		return nil, generation, nil
	}

	span.SetAttributes(attribute.Bool("cache.hit", true))
	var product entity.Product
	if err := json.Unmarshal([]byte(val), &product); err != nil {
		c.recordError(ctx, "get")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, "", fmt.Errorf("unmarshal cached product: %w", err)
	}
	return &product, generation, nil
}

func (c *productCache) Set(ctx context.Context, product *entity.Product, ttl time.Duration, generation string) error {
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
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
	if ttl < time.Millisecond || generation == "" {
		return errors.New("cache fill requires a positive TTL and generation")
	}
	if err := retry.Do(ctx, cacheRetryPolicy(), func() error {
		return c.client.Eval(ctx, `
if redis.call('GET', KEYS[2]) == ARGV[1] then
    redis.call('SET', KEYS[1], ARGV[2], 'PX', ARGV[3])
    return 1
end
return 0
`, []string{cacheKey(product.ID), generationKey(product.ID)}, generation, data, ttl.Milliseconds()).Err()
	}); err != nil {
		c.recordError(ctx, "set")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (c *productCache) Delete(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	ctx, span := tracer.Start(ctx, "cache.product.delete")
	defer span.End()
	span.SetAttributes(attribute.String("cache.key", cacheKey(id)))

	if err := retry.Do(ctx, cacheRetryPolicy(), func() error {
		return c.client.Eval(ctx, `
redis.call('SET', KEYS[2], ARGV[1], 'PX', 30000)
return redis.call('DEL', KEYS[1])
`, []string{cacheKey(id), generationKey(id)}, uuid.NewString()).Err()
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
	return "product:{" + id + "}"
}

func generationKey(id string) string {
	return cacheKey(id) + ":generation"
}

func cacheRetryPolicy() retry.Config {
	policy := retry.DefaultConfig()
	policy.Retryable = retry.IsTransient
	return policy
}
