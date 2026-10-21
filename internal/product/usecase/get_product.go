package usecase

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

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
	cached, generation, cacheErr := uc.cache.Get(ctx, id)
	if cacheErr == nil && cached != nil {
		res = response.FromEntity(cached)
		operation.Succeed()
		return res, nil
	}
	if cacheErr != nil {
		operation.Degrade("cache")
	}

	product, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get product: %w", err)
	}

	if cacheErr == nil {
		if err := uc.cache.Set(ctx, product, generation); err != nil {
			operation.Degrade("cache")
		}
	}

	res = response.FromEntity(product)
	operation.Succeed()
	return res, nil
}
