package cache_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

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
	return cache.NewProductCache(client)
}

func TestProductCache_SetAndGet(t *testing.T) {
	c := setup(t)

	p := &entity.Product{ID: "test-cache-1", Name: "Cached Widget", Price: 9.99, Stock: 5}
	if err := c.Set(context.Background(), p, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := c.Get(context.Background(), p.ID)
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

	got, err := c.Get(context.Background(), "test-cache-does-not-exist")
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
	if err := c.Set(context.Background(), p, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := c.Delete(context.Background(), p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := c.Get(context.Background(), p.ID)
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
	if err := c.Set(context.Background(), p, 50*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	got, err := c.Get(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("Get after TTL: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil after TTL expiry, got %+v", got)
		_ = c.Delete(context.Background(), p.ID)
	}
}
