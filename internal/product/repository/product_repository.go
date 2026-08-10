package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"gorm.io/gorm"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	domainrepo "github.com/boilerplate/microservice-boilerplate/internal/product/domain/repository"
)

var tracer = otel.Tracer("product/repository")

type productRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) domainrepo.ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) Create(ctx context.Context, product *entity.Product) error {
	ctx, span := tracer.Start(ctx, "product.repository.Create")
	defer span.End()

	if err := r.db.WithContext(ctx).Create(product).Error; err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("repository create product: %w", err)
	}
	span.SetAttributes(attribute.String("product.id", product.ID))
	return nil
}

func (r *productRepository) FindByID(ctx context.Context, id string) (*entity.Product, error) {
	ctx, span := tracer.Start(ctx, "product.repository.FindByID")
	defer span.End()
	span.SetAttributes(attribute.String("product.id", id))

	var product entity.Product
	if err := r.db.WithContext(ctx).First(&product, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entity.ErrProductNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("repository find product: %w", err)
	}
	return &product, nil
}

func (r *productRepository) FindAll(ctx context.Context) ([]entity.Product, error) {
	ctx, span := tracer.Start(ctx, "product.repository.FindAll")
	defer span.End()

	var products []entity.Product
	if err := r.db.WithContext(ctx).Find(&products).Error; err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("repository find all products: %w", err)
	}
	span.SetAttributes(attribute.Int("count", len(products)))
	return products, nil
}

func (r *productRepository) FindAllPaginated(ctx context.Context, params domainrepo.ListParams) ([]entity.Product, int64, error) {
	ctx, span := tracer.Start(ctx, "product.repository.FindAllPaginated")
	defer span.End()
	span.SetAttributes(
		attribute.Int("offset", params.Offset),
		attribute.Int("limit", params.Limit),
		attribute.String("search", params.Search),
		attribute.String("sort_by", params.SortBy),
		attribute.String("sort_dir", params.SortDir),
	)

	// base query shared by count + find, filters applied once
	query := r.db.WithContext(ctx).Model(&entity.Product{})
	if params.Search != "" {
		// portable substring search across drivers (postgres/mysql/sqlite/sqlserver).
		// ponytail: swap for Postgres tsvector + GIN index if ranked relevance is
		// ever needed; LIKE is fine for a boilerplate reference domain.
		like := "%" + strings.ToLower(params.Search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, fmt.Errorf("repository count products: %w", err)
	}

	findQuery := query.Session(&gorm.Session{}).Offset(params.Offset).Limit(params.Limit)
	if params.SortBy != "" {
		// caller (usecase) must have already validated SortBy/SortDir against
		// a whitelist — raw query-string values must never reach Order().
		findQuery = findQuery.Order(params.SortBy + " " + params.SortDir)
	}
	for _, rel := range params.Preload {
		// same whitelist contract as SortBy — see usecase.allowedPreloads.
		findQuery = findQuery.Preload(rel)
	}

	var products []entity.Product
	if err := findQuery.Find(&products).Error; err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, fmt.Errorf("repository find paginated products: %w", err)
	}
	span.SetAttributes(attribute.Int64("total", total))
	return products, total, nil
}

func (r *productRepository) Update(ctx context.Context, product *entity.Product) error {
	ctx, span := tracer.Start(ctx, "product.repository.Update")
	defer span.End()
	span.SetAttributes(attribute.String("product.id", product.ID))

	if err := r.db.WithContext(ctx).Save(product).Error; err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("repository update product: %w", err)
	}
	return nil
}

func (r *productRepository) Delete(ctx context.Context, id string) error {
	ctx, span := tracer.Start(ctx, "product.repository.Delete")
	defer span.End()
	span.SetAttributes(attribute.String("product.id", id))

	result := r.db.WithContext(ctx).Delete(&entity.Product{}, "id = ?", id)
	if result.Error != nil {
		span.RecordError(result.Error)
		span.SetStatus(codes.Error, result.Error.Error())
		return fmt.Errorf("repository delete product: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return entity.ErrProductNotFound
	}
	return nil
}
