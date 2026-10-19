package usecase

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/event"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/response"
)

var updateTracer = otel.Tracer("product/usecase")

type UpdateProductUseCase interface {
	Execute(ctx context.Context, id string, req request.UpdateProductRequest) (*response.ProductResponse, error)
}

type updateProductUseCase struct {
	repo      repository.ProductRepository
	cache     repository.ProductCache
	publisher event.Publisher
}

func NewUpdateProductUseCase(
	repo repository.ProductRepository,
	cache repository.ProductCache,
	publisher event.Publisher,
) UpdateProductUseCase {
	return &updateProductUseCase{repo: repo, cache: cache, publisher: publisher}
}

func (uc *updateProductUseCase) Execute(ctx context.Context, id string, req request.UpdateProductRequest) (*response.ProductResponse, error) {
	ctx, span := updateTracer.Start(ctx, "product.usecase.UpdateProduct")
	defer span.End()
	span.SetAttributes(attribute.String("product.id", id))

	product, err := uc.repo.FindByIDForUpdate(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("find product for update: %w", err)
	}

	product.Name = req.Name
	product.Description = req.Description
	product.Price = req.Price
	product.Stock = req.Stock

	if err := uc.repo.Update(ctx, product); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("update product: %w", err)
	}

	_ = uc.cache.Delete(ctx, id)

	evt := event.ProductUpdatedEvent{
		ProductID:   product.ID,
		Name:        product.Name,
		Description: product.Description,
		Price:       product.Price,
		Stock:       product.Stock,
		UpdatedAt:   product.UpdatedAt,
	}
	_ = uc.publisher.Publish(ctx, evt)

	return response.FromEntity(product), nil
}
