package repository_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/repository"
	"github.com/boilerplate/microservice-boilerplate/pkg/databases"
	"github.com/boilerplate/microservice-boilerplate/pkg/validation"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestProductListQueries(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "products.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := databases.Close(database); err != nil {
			t.Error(err)
		}
	})
	if err := database.AutoMigrate(&entity.Product{}); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewProductRepository(database)
	for _, product := range []entity.Product{
		{ID: "1", Name: "Alpha WIDGET", Price: 5, Stock: 0},
		{ID: "2", Name: "Beta widget", Price: 15, Stock: 5},
		{ID: "3", Name: "Other", Description: "A Widget", Price: 15, Stock: 10},
	} {
		if err := repo.Create(context.Background(), &product); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		name    string
		params  request.ListProductsRequest
		ids     []string
		total   int64
		invalid bool
	}{
		{name: "defaults", ids: []string{"1", "2", "3"}, total: 3},
		{name: "portable case insensitive search", params: request.ListProductsRequest{Search: "wIdGeT"}, ids: []string{"1", "2", "3"}, total: 3},
		{name: "numeric range", params: request.ListProductsRequest{Filters: []request.ProductFilter{{Field: "price", Operator: "IS_MORE_THAN_OR_EQUAL", Value: "10"}, {Field: "price", Operator: "IS_LESS_THAN_OR_EQUAL", Value: "20"}}}, ids: []string{"2", "3"}, total: 2},
		{name: "integer range", params: request.ListProductsRequest{Filters: []request.ProductFilter{{Field: "stock", Operator: "IS_MORE_THAN_OR_EQUAL", Value: "1"}, {Field: "stock", Operator: "IS_LESS_THAN_OR_EQUAL", Value: "10"}}}, ids: []string{"2", "3"}, total: 2},
		{name: "contains", params: request.ListProductsRequest{Filters: []request.ProductFilter{{Field: "name", Operator: "IS_CONTAIN", Value: "widget"}}}, ids: []string{"1", "2"}, total: 2},
		{name: "stable custom sort", params: request.ListProductsRequest{Page: 2, PageSize: 1, Sorts: []request.ProductSort{{Field: "price", Direction: "DESCENDING"}}}, ids: []string{"3"}, total: 3},
		{name: "stable default sort", params: request.ListProductsRequest{Page: 2, PageSize: 1}, ids: []string{"2"}, total: 3},
		{name: "empty page retains total", params: request.ListProductsRequest{Page: 10, PageSize: 1}, ids: []string{}, total: 3},
		{name: "maximum valid offset", params: request.ListProductsRequest{Page: 1001, PageSize: 100}, ids: []string{}, total: 3},
		{name: "value injection is parameterized", params: request.ListProductsRequest{Filters: []request.ProductFilter{{Field: "name", Operator: "IS_EQUAL", Value: "' OR 1=1 --"}}}, ids: []string{}},
		{name: "invalid value", params: request.ListProductsRequest{Filters: []request.ProductFilter{{Field: "price", Operator: "IS_EQUAL", Value: "no"}}}, invalid: true},
		{name: "nonfinite value", params: request.ListProductsRequest{Filters: []request.ProductFilter{{Field: "price", Operator: "IS_EQUAL", Value: "NaN"}}}, invalid: true},
		{name: "fractional stock", params: request.ListProductsRequest{Filters: []request.ProductFilter{{Field: "stock", Operator: "IS_EQUAL", Value: "1.5"}}}, invalid: true},
		{name: "invalid field", params: request.ListProductsRequest{Filters: []request.ProductFilter{{Field: "id", Operator: "IS_EQUAL", Value: "1"}}}, invalid: true},
		{name: "invalid operator", params: request.ListProductsRequest{Filters: []request.ProductFilter{{Field: "stock", Operator: "IS_CONTAIN", Value: "1"}}}, invalid: true},
		{name: "invalid sort", params: request.ListProductsRequest{Sorts: []request.ProductSort{{Field: "description", Direction: "ASCENDING"}}}, invalid: true},
		{name: "sort injection", params: request.ListProductsRequest{Sorts: []request.ProductSort{{Field: "id; DROP TABLE products", Direction: "ASCENDING"}}}, invalid: true},
		{name: "invalid direction", params: request.ListProductsRequest{Sorts: []request.ProductSort{{Field: "name", Direction: "sideways"}}}, invalid: true},
		{name: "invalid preload", params: request.ListProductsRequest{Preload: []string{"Category"}}, invalid: true},
		{name: "excessive page size", params: request.ListProductsRequest{PageSize: 101}, invalid: true},
		{name: "excessive offset", params: request.ListProductsRequest{Page: 1002, PageSize: 100}, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			products, total, err := repo.FindAllPaginated(context.Background(), test.params)
			if test.invalid {
				if !errors.Is(err, entity.ErrInvalidProductQuery) {
					t.Fatalf("error=%v, want invalid query", err)
				}
				if test.name == "invalid field" {
					var detail *validation.Error
					if !errors.As(err, &detail) || detail.Source == nil || !errors.Is(err, detail.Source) || strings.Contains(err.Error(), "filter field or operator not allowed") {
						t.Fatalf("generator cause not safely preserved: %v", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]string, len(products))
			for index := range products {
				ids[index] = products[index].ID
			}
			if !reflect.DeepEqual(ids, test.ids) || total != test.total {
				t.Fatalf("ids=%v total=%d, want ids=%v total=%d", ids, total, test.ids, test.total)
			}
		})
	}
}
