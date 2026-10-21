package usecase

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/response"
	pkglogger "github.com/boilerplate/microservice-boilerplate/pkg/logger"
	"github.com/boilerplate/microservice-boilerplate/pkg/validation"
)

var getTracer = otel.Tracer("product/usecase")

type GetProductUseCase interface {
	Execute(ctx context.Context, id string) (*response.ProductResponse, error)
}

type getProductUseCase struct {
	repo  repository.ProductRepository
	cache repository.ProductCache
	log   *zap.Logger
	loads singleflight.Group
}

type productLoad struct {
	product  entity.Product
	degraded bool
}

func NewGetProductUseCase(repo repository.ProductRepository, cache repository.ProductCache, log *zap.Logger) GetProductUseCase {
	return &getProductUseCase{repo: repo, cache: cache, log: log}
}

func (uc *getProductUseCase) Execute(ctx context.Context, id string) (res *response.ProductResponse, err error) {
	ctx, span := getTracer.Start(ctx, "product.usecase.GetProduct")
	defer span.End()
	span.SetAttributes(attribute.String("product.id", id))
	operation := pkglogger.StartOperation(ctx, uc.log, pkglogger.OperationConfig{
		Component: "product.usecase", Name: "GetProduct",
	}, map[string]any{"id": id})
	defer func() { operation.Finish(res, err) }()
	if err := validation.Required("id", id, entity.ErrInvalidProduct); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	operation.Debug("business validation passed")
	cached, _, cacheErr := uc.cache.Get(ctx, id)
	if cacheErr == nil && cached != nil {
		res = response.FromEntity(cached)
		operation.Succeed()
		return res, nil
	}
	if cacheErr != nil {
		operation.Degrade("cache")
	}
	if err := ctx.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	result := uc.loads.DoChan(id, func() (any, error) {
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		return uc.loadProduct(loadCtx, id)
	})
	select {
	case <-ctx.Done():
		span.RecordError(ctx.Err())
		span.SetStatus(codes.Error, ctx.Err().Error())
		return nil, ctx.Err()
	case loaded := <-result:
		if loaded.Err != nil {
			span.RecordError(loaded.Err)
			span.SetStatus(codes.Error, loaded.Err.Error())
			return nil, fmt.Errorf("get product: %w", loaded.Err)
		}
		value := loaded.Val.(productLoad)
		if value.degraded {
			operation.Degrade("cache")
		}
		res = response.FromEntity(&value.product)
	}

	operation.Succeed()
	return res, nil
}

func (uc *getProductUseCase) loadProduct(ctx context.Context, id string) (productLoad, error) {
	cached, generation, cacheErr := uc.cache.Get(ctx, id)
	if cacheErr == nil && cached != nil {
		return productLoad{product: *cached}, nil
	}
	product, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return productLoad{}, err
	}
	degraded := cacheErr != nil
	if cacheErr == nil {
		degraded = uc.cache.Set(ctx, product, generation) != nil
	}
	return productLoad{product: *product, degraded: degraded}, nil
}
