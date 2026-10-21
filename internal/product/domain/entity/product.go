package entity

import (
	"errors"
	"math"
	"time"

	"github.com/boilerplate/microservice-boilerplate/pkg/apperror"
	"github.com/boilerplate/microservice-boilerplate/pkg/validation"
)

var (
	ErrProductNotFound  = &apperror.Error{Code: "product_not_found", Message: "product not found"}
	ErrProductConflict  = &apperror.Error{Code: "product_conflict", Message: "product changed concurrently"}
	ErrInvalidProduct   = &apperror.Error{Code: "invalid_product", Message: "invalid product"}
	ErrInvalidSortField = errors.New("invalid sort field")
	ErrInvalidPreload   = errors.New("invalid preload relation")
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
