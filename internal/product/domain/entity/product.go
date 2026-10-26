package entity

import (
	"errors"
	"math"
	"time"

	"github.com/boilerplate/microservice-boilerplate/pkg/apperror"
	"github.com/boilerplate/microservice-boilerplate/pkg/validation"
)

var (
	ErrProductNotFound     = &apperror.Error{Code: "product_not_found", Message: "product not found"}
	ErrProductConflict     = &apperror.Error{Code: "product_conflict", Message: "product changed concurrently"}
	ErrInvalidProduct      = &apperror.Error{Code: "invalid_product", Message: "invalid product"}
	ErrInvalidProductQuery = &apperror.Error{Code: "invalid_query", Message: "invalid list query"}
	ErrInvalidSortField    = errors.New("invalid sort field")
	ErrInvalidPreload      = errors.New("invalid preload relation")
)

type Product struct {
	ID          string `gorm:"primaryKey;type:varchar(36)"`
	Name        string `gorm:"not null"`
	Description string
	Price       float64 `gorm:"not null"`
	Stock       int     `gorm:"not null;default:0"`
	Version     int64   `gorm:"not null;default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProductDetails struct {
	Name        string
	Description string
	Price       float64
	Stock       int
}

func NewProduct(id string, details ProductDetails) (*Product, error) {
	if err := validation.Required("id", id, ErrInvalidProduct); err != nil {
		return nil, err
	}
	product := &Product{ID: id}
	if err := product.ChangeDetails(details); err != nil {
		return nil, err
	}
	return product, nil
}

func (product *Product) ChangeDetails(details ProductDetails) error {
	if err := ValidateProduct(details.Name, details.Price, details.Stock); err != nil {
		return err
	}
	product.Name = details.Name
	product.Description = details.Description
	product.Price = details.Price
	product.Stock = details.Stock
	return nil
}

func ValidateProduct(name string, price float64, stock int) error {
	if err := validation.Required("name", name, ErrInvalidProduct); err != nil {
		return err
	}
	switch {
	case math.IsNaN(price) || math.IsInf(price, 0) || price <= 0:
		return &validation.Error{Field: "price", Rule: "finite_positive", Cause: ErrInvalidProduct}
	case stock < 0:
		return &validation.Error{Field: "stock", Rule: "non_negative", Cause: ErrInvalidProduct}
	default:
		return nil
	}
}
