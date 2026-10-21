package usecase_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/response"
	"github.com/boilerplate/microservice-boilerplate/internal/product/usecase"
	"go.uber.org/zap"
)

type observedContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (ctx *observedContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.observed) })
	return ctx.Context.Done()
}

type blockingProductRepository struct {
	repository.ProductRepository
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (repo *blockingProductRepository) FindByID(ctx context.Context, id string) (*entity.Product, error) {
	if repo.calls.Add(1) == 1 {
		close(repo.started)
	}
	select {
	case <-repo.release:
		return &entity.Product{ID: id, Name: "Shared", Price: 1}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type concurrentMissCache struct {
	setCalls atomic.Int32
	getErr   error
	setErr   error
}

func (cache *concurrentMissCache) Get(context.Context, string) (*entity.Product, string, error) {
	return nil, "generation", cache.getErr
}

func (cache *concurrentMissCache) Set(context.Context, *entity.Product, string) error {
	cache.setCalls.Add(1)
	return cache.setErr
}

func (cache *concurrentMissCache) Delete(context.Context, string) error {
	return nil
}

type productResult struct {
	product *response.ProductResponse
	err     error
}

func TestConcurrentMissesShareLoadDespiteCanceledLeader(t *testing.T) {
	for _, cacheErr := range []error{nil, errors.New("Redis unavailable")} {
		t.Run(map[bool]string{true: "cache unavailable", false: "cache miss"}[cacheErr != nil], func(t *testing.T) {
			deadline, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			repo := &blockingProductRepository{started: make(chan struct{}), release: make(chan struct{})}
			cache := &concurrentMissCache{getErr: cacheErr}
			get := usecase.NewGetProductUseCase(repo, cache, zap.NewNop())
			leaderCtx, cancelLeader := context.WithCancel(deadline)
			defer cancelLeader()
			leader := make(chan productResult, 1)
			go func() {
				product, err := get.Execute(leaderCtx, "product-1")
				leader <- productResult{product, err}
			}()
			select {
			case <-repo.started:
			case <-deadline.Done():
				t.Fatal("repository load did not start")
			}
			followerCtx := &observedContext{Context: deadline, observed: make(chan struct{})}
			follower := make(chan productResult, 1)
			go func() {
				product, err := get.Execute(followerCtx, "product-1")
				follower <- productResult{product, err}
			}()
			select {
			case <-followerCtx.observed:
			case <-deadline.Done():
				t.Fatal("follower did not join shared load")
			}
			cancelLeader()
			select {
			case result := <-leader:
				if !errors.Is(result.err, context.Canceled) {
					t.Fatalf("leader error = %v, want cancellation", result.err)
				}
			case <-deadline.Done():
				t.Fatal("canceled leader remained blocked")
			}
			close(repo.release)
			select {
			case result := <-follower:
				if result.err != nil || result.product == nil || result.product.ID != "product-1" {
					t.Fatalf("unexpected follower result: %+v", result)
				}
			case <-deadline.Done():
				t.Fatal("follower did not finish")
			}
			if repo.calls.Load() != 1 {
				t.Fatalf("repository calls = %d, want one shared load", repo.calls.Load())
			}
			wantSets := int32(1)
			if cacheErr != nil {
				wantSets = 0
			}
			if cache.setCalls.Load() != wantSets {
				t.Fatalf("cache fills = %d, want %d", cache.setCalls.Load(), wantSets)
			}
		})
	}
}

func TestCanceledReadDoesNotStartSharedLoad(t *testing.T) {
	repo := &blockingProductRepository{started: make(chan struct{}), release: make(chan struct{})}
	get := usecase.NewGetProductUseCase(repo, &concurrentMissCache{}, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := get.Execute(ctx, "product-1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want cancellation", err)
	}
	if repo.calls.Load() != 0 {
		t.Fatal("canceled caller started a database load")
	}
}
