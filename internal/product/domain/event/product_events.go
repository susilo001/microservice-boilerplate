package event

import "time"

type ProductCreatedEvent struct {
	EventID       string    `json:"event_id"`
	SchemaVersion int       `json:"schema_version"`
	Version       int64     `json:"version"`
	ProductID     string    `json:"product_id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Price         float64   `json:"price"`
	Stock         int       `json:"stock"`
	CreatedAt     time.Time `json:"created_at"`
}

func (e ProductCreatedEvent) EventName() string     { return "product.created" }
func (e ProductCreatedEvent) OccurredAt() time.Time { return e.CreatedAt }

type ProductUpdatedEvent struct {
	EventID       string    `json:"event_id"`
	SchemaVersion int       `json:"schema_version"`
	Version       int64     `json:"version"`
	ProductID     string    `json:"product_id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Price         float64   `json:"price"`
	Stock         int       `json:"stock"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (e ProductUpdatedEvent) EventName() string     { return "product.updated" }
func (e ProductUpdatedEvent) OccurredAt() time.Time { return e.UpdatedAt }

func (e ProductCreatedEvent) AggregateID() string { return e.ProductID }
func (e ProductUpdatedEvent) AggregateID() string { return e.ProductID }
