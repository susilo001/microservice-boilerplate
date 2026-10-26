package request

import (
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/pkg/validation"
)

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

// ListProductsRequest carries structured input parsed from field:value:operator
// filters and field:direction sorts. The repository owns the SQL allow-lists.
type ListProductsRequest struct {
	Page     int
	PageSize int
	Search   string
	Filters  []ProductFilter
	Sorts    []ProductSort
	Preload  []string
}

type ProductFilter struct {
	Field    string
	Operator ProductFilterOperator
	Value    any
}

type ProductSort struct {
	Field     string
	Direction ProductSortDirection
}

type ProductFilterOperator string

const (
	ProductFilterEqual              ProductFilterOperator = "IS_EQUAL"
	ProductFilterContains           ProductFilterOperator = "IS_CONTAIN"
	ProductFilterGreaterThanOrEqual ProductFilterOperator = "IS_MORE_THAN_OR_EQUAL"
	ProductFilterLessThanOrEqual    ProductFilterOperator = "IS_LESS_THAN_OR_EQUAL"
)

type ProductSortDirection string

const (
	ProductSortAscending  ProductSortDirection = "ASCENDING"
	ProductSortDescending ProductSortDirection = "DESCENDING"
)

func (req ListProductsRequest) Normalize() (ListProductsRequest, error) {
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		return ListProductsRequest{}, &validation.Error{Field: "page_size", Rule: "between_1_and_100", Cause: entity.ErrInvalidProductQuery}
	}
	if req.Page < 1 || req.Page-1 > 100000/req.PageSize {
		return ListProductsRequest{}, &validation.Error{Field: "page", Rule: "bounded_offset", Cause: entity.ErrInvalidProductQuery}
	}
	return req, nil
}
