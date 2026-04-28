package event

import "time"

type ProductCreatedEvent struct {
	ProductID   string    `json:"product_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Stock       int       `json:"stock"`
	CreatedAt   time.Time `json:"created_at"`
}

func (e ProductCreatedEvent) EventName() string     { return "product.created" }
func (e ProductCreatedEvent) OccurredAt() time.Time { return e.CreatedAt }

type ProductUpdatedEvent struct {
	ProductID   string    `json:"product_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Stock       int       `json:"stock"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (e ProductUpdatedEvent) EventName() string     { return "product.updated" }
func (e ProductUpdatedEvent) OccurredAt() time.Time { return e.UpdatedAt }
