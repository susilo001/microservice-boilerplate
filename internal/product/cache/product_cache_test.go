package cache_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	pkglogger "github.com/boilerplate/microservice-boilerplate/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/boilerplate/microservice-boilerplate/internal/product/cache"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	domainrepo "github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
)

func setup(t *testing.T) domainrepo.ProductCache {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set — skipping cache integration test")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { client.Close() })
	return cache.NewProductCache(client, zap.NewNop())
}

func TestCacheFailureEmitsOneCorrelatedWarning(t *testing.T) {
	core, entries := observer.New(zap.WarnLevel)
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
	t.Cleanup(func() { client.Close() })
	cache := cache.NewProductCache(client, zap.New(core))
	ctx, cancel := context.WithCancel(pkglogger.WithCorrelationID(context.Background(), "cache-request"))
	cancel()
	_, _, err := cache.Get(ctx, "product")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want cancellation", err)
	}
	logs := entries.FilterMessage("cache operation failed").All()
	if len(logs) != 1 || logs[0].ContextMap()["correlationid"] != "cache-request" ||
		logs[0].ContextMap()["operation"] != "get" {
		t.Fatalf("unexpected failure warnings: %+v", logs)
	}
}

func TestProductCache_SetAndGet(t *testing.T) {
	c := setup(t)

	p := &entity.Product{ID: "test-cache-1", Name: "Cached Widget", Price: 9.99, Stock: 5}
	if err := fill(context.Background(), c, p); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, _, err := c.Get(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get: expected product, got nil")
	}
	if got.Name != p.Name {
		t.Errorf("Name = %q, want %q", got.Name, p.Name)
	}
	if got.Price != p.Price {
		t.Errorf("Price = %v, want %v", got.Price, p.Price)
	}

	_ = c.Delete(context.Background(), p.ID)
}

func TestProductCache_Get_Miss(t *testing.T) {
	c := setup(t)

	got, _, err := c.Get(context.Background(), "test-cache-does-not-exist")
	if err != nil {
		t.Fatalf("Get miss: unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("Get miss: expected nil, got %+v", got)
	}
}

func TestProductCache_Delete(t *testing.T) {
	c := setup(t)

	p := &entity.Product{ID: "test-cache-del-1", Name: "Doomed", Price: 1.00, Stock: 1}
	if err := fill(context.Background(), c, p); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := c.Delete(context.Background(), p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, _, err := c.Get(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if got != nil {
		t.Errorf("Get after delete: expected nil, got %+v", got)
	}
}

func TestProductCache_TTL(t *testing.T) {
	c := setup(t)

	p := &entity.Product{ID: "test-cache-ttl-1", Name: "Expiring", Price: 0.01, Stock: 1}
	if err := fill(context.Background(), c, p); err != nil {
		t.Fatalf("Set: %v", err)
	}

	client := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR")})
	t.Cleanup(func() { client.Close() })
	t.Cleanup(func() { _ = c.Delete(context.Background(), p.ID) })
	ttl, err := client.PTTL(context.Background(), "product:{"+p.ID+"}").Result()
	if err != nil {
		t.Fatalf("PTTL: %v", err)
	}
	if ttl <= 0 || ttl > 5*time.Minute || ttl < 5*time.Minute-5*time.Second {
		t.Errorf("TTL = %v, want approximately 5 minutes", ttl)
	}
}

func fill(ctx context.Context, cache domainrepo.ProductCache, product *entity.Product) error {
	_, generation, err := cache.Get(ctx, product.ID)
	if err != nil {
		return err
	}
	return cache.Set(ctx, product, generation)
}

func TestProductCacheRejectsFillAfterInvalidation(t *testing.T) {
	cache := setup(t)
	ctx := context.Background()
	product := &entity.Product{ID: "test-cache-generation", Name: "Stale"}
	_, generation, err := cache.Get(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Delete(ctx, product.ID); err != nil {
		t.Fatal(err)
	}
	if err := cache.Set(ctx, product, generation); err != nil {
		t.Fatal(err)
	}
	got, _, err := cache.Get(ctx, product.ID)
	if err != nil || got != nil {
		t.Fatalf("stale fill accepted: product=%+v, error=%v", got, err)
	}
}
