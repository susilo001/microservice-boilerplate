package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/event"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/response"
	pkgevent "github.com/boilerplate/microservice-boilerplate/pkg/event"
	pkglogger "github.com/boilerplate/microservice-boilerplate/pkg/logger"
	"github.com/boilerplate/microservice-boilerplate/pkg/validation"
)

var updateTracer = otel.Tracer("product/usecase")

type UpdateProductUseCase interface {
	Execute(ctx context.Context, id string, req request.UpdateProductRequest) (*response.ProductResponse, error)
}

type updateProductUseCase struct {
	repo      repository.ProductRepository
	cache     repository.ProductCache
	publisher pkgevent.Publisher
	log       *zap.Logger
}

func NewUpdateProductUseCase(
	repo repository.ProductRepository,
	cache repository.ProductCache,
	publisher pkgevent.Publisher,
	log *zap.Logger,
) UpdateProductUseCase {
	return &updateProductUseCase{repo: repo, cache: cache, publisher: publisher, log: log}
}

func (uc *updateProductUseCase) Execute(ctx context.Context, id string, req request.UpdateProductRequest) (res *response.ProductResponse, err error) {
	ctx, span := updateTracer.Start(ctx, "product.usecase.UpdateProduct")
	defer span.End()
	span.SetAttributes(attribute.String("product.id", id))
	operation := pkglogger.StartOperation(ctx, uc.log, pkglogger.OperationConfig{Component: "product.usecase", Name: "UpdateProduct"}, map[string]any{"id": id, "product": req})
	defer func() { operation.Finish(res, err) }()
	if err := validation.Required("id", id, entity.ErrInvalidProduct); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if err := entity.ValidateProduct(req.Name, req.Price, req.Stock); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	operation.Debug("business validation passed")
	product, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("find product for update: %w", err)
	}

	if err := product.ChangeDetails(entity.ProductDetails{
		Name: req.Name, Description: req.Description, Price: req.Price, Stock: req.Stock,
	}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if err := uc.repo.Update(ctx, product); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("update product: %w", err)
	}

	if err := uc.cache.Delete(ctx, id); err != nil {
		operation.Degrade("cache")
	}

	evt := event.ProductUpdatedEvent{
		EventID:       uuid.NewString(),
		SchemaVersion: 1,
		Version:       product.Version,
		ProductID:     product.ID,
		Name:          product.Name,
		Description:   product.Description,
		Price:         product.Price,
		Stock:         product.Stock,
		UpdatedAt:     product.UpdatedAt,
	}
	if err := uc.publisher.Publish(ctx, evt); err != nil {
		operation.Degrade("publisher")
	}

	res = response.FromEntity(product)
	operation.Succeed()
	return res, nil
}
