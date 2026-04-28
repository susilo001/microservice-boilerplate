package response

import (
	"time"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
)

type ProductResponse struct {
	ID          string    `json:"id"          example:"550e8400-e29b-41d4-a716-446655440000"`
	Name        string    `json:"name"        example:"Wireless Keyboard"`
	Description string    `json:"description" example:"Compact mechanical keyboard"`
	Price       float64   `json:"price"       example:"49.99"`
	Stock       int       `json:"stock"       example:"100"`
	CreatedAt   time.Time `json:"created_at"  example:"2026-01-15T09:00:00Z"`
	UpdatedAt   time.Time `json:"updated_at"  example:"2026-04-21T12:30:00Z"`
}

// ErrorResponse is the standard error body returned by API handlers.
type ErrorResponse struct {
	Error string `json:"error" example:"something went wrong"`
}

func FromEntity(p *entity.Product) *ProductResponse {
	return &ProductResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Stock:       p.Stock,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}
