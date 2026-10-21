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
	pkglogger "github.com/boilerplate/microservice-boilerplate/pkg/logger"
	"github.com/boilerplate/microservice-boilerplate/pkg/validation"
)

var deleteTracer = otel.Tracer("product/usecase")

type DeleteProductUseCase interface {
	Execute(ctx context.Context, id string) error
}

type deleteProductUseCase struct {
	repo  repository.ProductRepository
	cache repository.ProductCache
	log   *zap.Logger
}

func NewDeleteProductUseCase(repo repository.ProductRepository, cache repository.ProductCache, log *zap.Logger) DeleteProductUseCase {
	return &deleteProductUseCase{repo: repo, cache: cache, log: log}
}

func (uc *deleteProductUseCase) Execute(ctx context.Context, id string) (err error) {
	ctx, span := deleteTracer.Start(ctx, "product.usecase.DeleteProduct")
	defer span.End()
	span.SetAttributes(attribute.String("product.id", id))
	operation := pkglogger.StartOperation(ctx, uc.log, pkglogger.OperationConfig{
		Component: "product.usecase", Name: "DeleteProduct",
	}, map[string]any{"id": id})
	defer func() { operation.Finish(map[string]any{"id": id, "deleted": err == nil}, err) }()
	if err := validation.Required("id", id, entity.ErrInvalidProduct); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	operation.Debug("business validation passed")
	if err := uc.repo.Delete(ctx, id); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("delete product: %w", err)
	}
	if err := uc.cache.Delete(ctx, id); err != nil {
		operation.Degrade("cache")
	}
	operation.Succeed()
	return nil
}
