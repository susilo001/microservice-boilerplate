package request

type CreateProductRequest struct {
	Name        string  `json:"name"        binding:"required"      example:"Wireless Keyboard"`
	Description string  `json:"description"                          example:"Compact mechanical keyboard"`
	Price       float64 `json:"price"       binding:"required,gt=0"  example:"49.99"`
	Stock       int     `json:"stock"       binding:"gte=0"          example:"100"`
}

type UpdateProductRequest struct {
	Name        string  `json:"name"        binding:"required"      example:"Wireless Keyboard Pro"`
	Description string  `json:"description"                          example:"Updated description"`
	Price       float64 `json:"price"       binding:"required,gt=0"  example:"59.99"`
	Stock       int     `json:"stock"       binding:"gte=0"          example:"75"`
}

// ListProductsRequest is product list's public query contract. Filter and
// sort use field:operator:value and field:direction forms. Tags are explicit
// allow-lists for SQL identifiers, never inferred from entity fields.
type ListProductsRequest struct {
	Page     int      `query:"page,default=1,min=1"`
	PageSize int      `query:"page_size,default=20,min=1"`
	Search   string   `query:"search" search:"name,description"`
	Filters  []string `query:"filter" filter:"name:string:eq|like,price:float64:eq|gte|lte,stock:int:eq|gte|lte"`
	Sort     string   `query:"sort" sort:"name,price,stock,created_at,updated_at"`
	Preload  []string `query:"preload,list" preload:""`
}
