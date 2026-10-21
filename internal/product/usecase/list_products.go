package usecase

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/response"
	pkglogger "github.com/boilerplate/microservice-boilerplate/pkg/logger"
	"github.com/boilerplate/microservice-boilerplate/pkg/queryutil"
)

var listTracer = otel.Tracer("product/usecase")

type ListProductsUseCase interface {
	Execute(ctx context.Context, req request.ListProductsRequest) ([]response.ProductResponse, int64, error)
}

type listProductsUseCase struct {
	repo repository.ProductRepository
	log  *zap.Logger
}

func NewListProductsUseCase(repo repository.ProductRepository, log *zap.Logger) ListProductsUseCase {
	return &listProductsUseCase{repo: repo, log: log}
}

func (uc *listProductsUseCase) Execute(ctx context.Context, req request.ListProductsRequest) (responses []response.ProductResponse, total int64, err error) {
	ctx, span := listTracer.Start(ctx, "product.usecase.ListProducts")
	defer span.End()
	operation := pkglogger.StartOperation(ctx, uc.log, pkglogger.OperationConfig{Component: "product.usecase", Name: "ListProducts"}, req)
	defer func() { operation.Finish(map[string]any{"list": responses, "total": total}, err) }()
	span.SetAttributes(
		attribute.Int("page", req.Page),
		attribute.Int("page_size", req.PageSize),
		attribute.String("search", req.Search),
		attribute.String("sort", req.Sort),
	)

	if _, err := queryutil.BuildListParams(req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, err
	}

	operation.Debug("business validation passed")
	products, total, err := uc.repo.FindAllPaginated(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, fmt.Errorf("list products: %w", err)
	}

	span.SetAttributes(attribute.Int64("total", total))
	responses = make([]response.ProductResponse, len(products))
	for i := range products {
		responses[i] = *response.FromEntity(&products[i])
	}
	operation.Succeed()
	return responses, total, nil
}
