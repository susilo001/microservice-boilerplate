package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/event"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/response"
)

var createTracer = otel.Tracer("product/usecase")

type CreateProductUseCase interface {
	Execute(ctx context.Context, req request.CreateProductRequest) (*response.ProductResponse, error)
}

type createProductUseCase struct {
	repo      repository.ProductRepository
	publisher event.Publisher
}

func NewCreateProductUseCase(repo repository.ProductRepository, publisher event.Publisher) CreateProductUseCase {
	return &createProductUseCase{repo: repo, publisher: publisher}
}

func (uc *createProductUseCase) Execute(ctx context.Context, req request.CreateProductRequest) (*response.ProductResponse, error) {
	ctx, span := createTracer.Start(ctx, "product.usecase.CreateProduct")
	defer span.End()

	product := &entity.Product{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
	}

	if err := uc.repo.Create(ctx, product); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("create product: %w", err)
	}

	span.SetAttributes(attribute.String("product.id", product.ID))

	evt := event.ProductCreatedEvent{
		ProductID:   product.ID,
		Name:        product.Name,
		Description: product.Description,
		Price:       product.Price,
		Stock:       product.Stock,
		CreatedAt:   product.CreatedAt,
	}
	// non-fatal: publish failure does not roll back the create
	_ = uc.publisher.Publish(ctx, evt)

	return response.FromEntity(product), nil
}
