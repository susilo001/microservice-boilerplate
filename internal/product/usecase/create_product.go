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
)

var createTracer = otel.Tracer("product/usecase")

type CreateProductUseCase interface {
	Execute(ctx context.Context, req request.CreateProductRequest) (*response.ProductResponse, error)
}

type createProductUseCase struct {
	repo      repository.ProductRepository
	publisher pkgevent.Publisher
	log       *zap.Logger
}

func NewCreateProductUseCase(repo repository.ProductRepository, publisher pkgevent.Publisher, log *zap.Logger) CreateProductUseCase {
	return &createProductUseCase{repo: repo, publisher: publisher, log: log}
}

func (uc *createProductUseCase) Execute(ctx context.Context, req request.CreateProductRequest) (res *response.ProductResponse, err error) {
	ctx, span := createTracer.Start(ctx, "product.usecase.CreateProduct")
	defer span.End()
	operation := pkglogger.StartOperation(ctx, uc.log, pkglogger.OperationConfig{
		Component: "product.usecase", Name: "CreateProduct",
	}, req)
	defer func() { operation.Finish(res, err) }()
	product, err := entity.NewProduct(uuid.NewString(), entity.ProductDetails{
		Name: req.Name, Description: req.Description, Price: req.Price, Stock: req.Stock,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	operation.Debug("business validation passed")
	if err := uc.repo.Create(ctx, product); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("create product: %w", err)
	}

	span.SetAttributes(attribute.String("product.id", product.ID))

	evt := event.ProductCreatedEvent{
		EventID:       uuid.NewString(),
		SchemaVersion: 1,
		Version:       product.Version,
		ProductID:     product.ID,
		Name:          product.Name,
		Description:   product.Description,
		Price:         product.Price,
		Stock:         product.Stock,
		CreatedAt:     product.CreatedAt,
	}
	// non-fatal: publish failure does not roll back the create
	if err := uc.publisher.Publish(ctx, evt); err != nil {
		operation.Degrade("publisher")
	}

	res = response.FromEntity(product)
	operation.Succeed()
	return res, nil
}
