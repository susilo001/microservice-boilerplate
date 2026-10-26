package repository

import (
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	sqlgenerator "github.com/susilo001/sql-generator"
	"github.com/susilo001/sql-generator/model"
)

func productListGenerator() *sqlgenerator.Generator {
	return &sqlgenerator.Generator{
		StrictQueries: true, PortableSearch: true, TieBreakerField: "id",
		MaxPageSize: 100, MaxOffset: 100000, MaxFiltersPerQuery: 20, MaxSortsPerQuery: 5,
		Schema: &sqlgenerator.ModelMeta{
			AllowedPreloads: map[string]bool{},
			AllowedSorts:    map[string]bool{"name": true, "price": true, "stock": true, "created_at": true, "updated_at": true},
			Fields: map[string]sqlgenerator.FieldMeta{
				"id":          {Column: "id"},
				"name":        {Column: "name", Searchable: true, ValueType: "string", Operators: map[model.Operator]bool{model.IsEqual: true, model.IsContain: true}},
				"description": {Column: "description", Searchable: true},
				"price":       {Column: "price", ValueType: "float64", Operators: map[model.Operator]bool{model.IsEqual: true, model.IsMoreThanOrEqual: true, model.IsLessThanOrEqual: true}},
				"stock":       {Column: "stock", ValueType: "int", Operators: map[model.Operator]bool{model.IsEqual: true, model.IsMoreThanOrEqual: true, model.IsLessThanOrEqual: true}},
				"created_at":  {Column: "created_at"},
				"updated_at":  {Column: "updated_at"},
			},
		},
	}
}

func productListQuery(params request.ListProductsRequest) (model.Query, error) {
	params, err := params.Normalize()
	if err != nil {
		return model.Query{}, err
	}
	query := model.Query{Search: params.Search}
	query.SelectParameter.PageDescriptor = model.Pagination{Page: params.Page, PageSize: params.PageSize}
	query.SelectParameter.Preloads = params.Preload
	for _, filter := range params.Filters {
		query.SelectParameter.Filters = append(query.SelectParameter.Filters, model.Filter{
			FieldName: filter.Field, Operator: model.Operator(filter.Operator), Value: filter.Value,
		})
	}
	for _, sort := range params.Sorts {
		query.SelectParameter.Sorts = append(query.SelectParameter.Sorts, model.Sort{
			FieldName: sort.Field, SortDirection: model.SortDirection(sort.Direction),
		})
	}
	return query, nil
}
