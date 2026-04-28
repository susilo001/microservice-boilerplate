package usecase

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
)

var deleteTracer = otel.Tracer("product/usecase")

type DeleteProductUseCase interface {
	Execute(ctx context.Context, id string) error
}

type deleteProductUseCase struct {
	repo  repository.ProductRepository
	cache repository.ProductCache
}

func NewDeleteProductUseCase(repo repository.ProductRepository, cache repository.ProductCache) DeleteProductUseCase {
	return &deleteProductUseCase{repo: repo, cache: cache}
}

func (uc *deleteProductUseCase) Execute(ctx context.Context, id string) error {
	ctx, span := deleteTracer.Start(ctx, "product.usecase.DeleteProduct")
	defer span.End()
	span.SetAttributes(attribute.String("product.id", id))

	if err := uc.repo.Delete(ctx, id); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("delete product: %w", err)
	}
	_ = uc.cache.Delete(ctx, id)
	return nil
}
