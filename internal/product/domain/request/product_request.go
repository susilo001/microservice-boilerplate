package request

type CreateProductRequest struct {
	Name        string  `json:"name"        validate:"required"     example:"Wireless Keyboard"`
	Description string  `json:"description"                          example:"Compact mechanical keyboard"`
	Price       float64 `json:"price"       validate:"required,gt=0" example:"49.99"`
	Stock       int     `json:"stock"       validate:"gte=0"         example:"100"`
}

type UpdateProductRequest struct {
	Name        string  `json:"name"        validate:"required"     example:"Wireless Keyboard Pro"`
	Description string  `json:"description"                          example:"Updated description"`
	Price       float64 `json:"price"       validate:"required,gt=0" example:"59.99"`
	Stock       int     `json:"stock"       validate:"gte=0"         example:"75"`
}
