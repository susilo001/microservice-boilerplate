package usecase

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/response"
)

var listTracer = otel.Tracer("product/usecase")

// allowedSortFields whitelists columns usable in ORDER BY. Never pass a
// raw query-string value into GORM's Order() — it isn't parameterized
// like Where() placeholders are.
var allowedSortFields = map[string]bool{
	"name":       true,
	"price":      true,
	"stock":      true,
	"created_at": true,
	"updated_at": true,
}

// allowedPreloads whitelists relation names usable in Preload(). Empty
// today because entity.Product has no relations.
// ponytail: add entries here (e.g. "Category") once a relation exists;
// until then any ?preload= value is rejected with ErrInvalidPreload.
var allowedPreloads = map[string]bool{}

type ListProductsUseCase interface {
	Execute(ctx context.Context, req request.ListProductsRequest) ([]response.ProductResponse, int64, error)
}

type listProductsUseCase struct {
	repo repository.ProductRepository
}

func NewListProductsUseCase(repo repository.ProductRepository) ListProductsUseCase {
	return &listProductsUseCase{repo: repo}
}

func (uc *listProductsUseCase) Execute(ctx context.Context, req request.ListProductsRequest) ([]response.ProductResponse, int64, error) {
	ctx, span := listTracer.Start(ctx, "product.usecase.ListProducts")
	defer span.End()
	span.SetAttributes(
		attribute.Int("page", req.Page),
		attribute.Int("page_size", req.PageSize),
		attribute.String("search", req.Search),
		attribute.String("sort_by", req.SortBy),
	)

	sortBy := ""
	sortDir := "asc"
	if req.SortBy != "" {
		if !allowedSortFields[req.SortBy] {
			err := entity.ErrInvalidSortField
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, 0, err
		}
		sortBy = req.SortBy
		if req.SortDir == "desc" {
			sortDir = "desc"
		}
	}

	for _, rel := range req.Preload {
		if !allowedPreloads[rel] {
			err := entity.ErrInvalidPreload
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, 0, err
		}
	}

	offset := (req.Page - 1) * req.PageSize
	products, total, err := uc.repo.FindAllPaginated(ctx, repository.ListParams{
		Offset:  offset,
		Limit:   req.PageSize,
		Search:  req.Search,
		SortBy:  sortBy,
		SortDir: sortDir,
		Preload: req.Preload,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, fmt.Errorf("list products: %w", err)
	}

	span.SetAttributes(attribute.Int64("total", total))
	responses := make([]response.ProductResponse, len(products))
	for i := range products {
		responses[i] = *response.FromEntity(&products[i])
	}
	return responses, total, nil
}
