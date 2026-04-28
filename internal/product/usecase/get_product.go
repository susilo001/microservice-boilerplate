package usecase

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/response"
)

const productCacheTTL = 5 * time.Minute

var getTracer = otel.Tracer("product/usecase")

type GetProductUseCase interface {
	Execute(ctx context.Context, id string) (*response.ProductResponse, error)
}

type getProductUseCase struct {
	repo  repository.ProductRepository
	cache repository.ProductCache
}

func NewGetProductUseCase(repo repository.ProductRepository, cache repository.ProductCache) GetProductUseCase {
	return &getProductUseCase{repo: repo, cache: cache}
}

func (uc *getProductUseCase) Execute(ctx context.Context, id string) (*response.ProductResponse, error) {
	ctx, span := getTracer.Start(ctx, "product.usecase.GetProduct")
	defer span.End()
	span.SetAttributes(attribute.String("product.id", id))

	if cached, err := uc.cache.Get(ctx, id); err == nil && cached != nil {
		return response.FromEntity(cached), nil
	}

	product, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get product: %w", err)
	}

	_ = uc.cache.Set(ctx, product, productCacheTTL)

	return response.FromEntity(product), nil
}
