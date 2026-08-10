package entity

import (
	"errors"
	"time"
)

var (
	ErrProductNotFound  = errors.New("product not found")
	ErrInvalidSortField = errors.New("invalid sort field")
	ErrInvalidPreload   = errors.New("invalid preload relation")
)

type Product struct {
	ID          string    `gorm:"primaryKey;type:varchar(36)"`
	Name        string    `gorm:"not null"`
	Description string
	Price       float64   `gorm:"not null"`
	Stock       int       `gorm:"not null;default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
