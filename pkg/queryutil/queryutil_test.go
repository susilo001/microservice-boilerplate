package queryutil_test

import (
	"errors"
	"net/url"
	"strconv"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/boilerplate/microservice-boilerplate/pkg/queryutil"
	"github.com/boilerplate/microservice-boilerplate/pkg/validation"
)

type widget struct {
	ID    string `gorm:"primaryKey"`
	Name  string
	Price float64
}

type listRequest struct {
	Page     int      `query:"page,default=1,min=1"`
	PageSize int      `query:"page_size,default=20,min=1"`
	Search   string   `query:"search" search:"name"`
	Filters  []string `query:"filter" filter:"name:string:eq|like,price:float64:eq|gte|lte"`
	Sort     string   `query:"sort" sort:"name,price" sortkey:"id"`
	Preload  []string `query:"preload,list" preload:""`
}

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&widget{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, w := range []widget{{ID: "1", Name: "Alpha", Price: 5}, {ID: "2", Name: "Beta", Price: 15}, {ID: "3", Name: "Gamma", Price: 50}} {
		if err := db.Create(&w).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return db
}

func TestBindAndBuildListParams(t *testing.T) {
	values := url.Values{
		"search":    {"beta"},
		"filter":    {"price:gte:10", "price:lte:20"},
		"sort":      {"price:desc"},
		"page":      {"2"},
		"page_size": {"5"},
	}
	var req listRequest
	if err := queryutil.Bind(values, &req); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	params, err := queryutil.BuildListParams(req)
	if err != nil {
		t.Fatalf("BuildListParams: %v", err)
	}
	if params.Search != "beta" || len(params.Filters) != 2 || params.SortBy != "price" || params.SortDir != "desc" || params.Offset != 5 || params.Limit != 5 {
		t.Errorf("params = %+v, want parsed search/filter/sort/page", params)
	}
}

func TestApplySearchAndFilters(t *testing.T) {
	db := setupDB(t)
	var req listRequest
	if err := queryutil.Bind(url.Values{"search": {"beta"}, "filter": {"price:gte:10", "price:lte:20"}}, &req); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	params, err := queryutil.BuildListParams(req)
	if err != nil {
		t.Fatalf("BuildListParams: %v", err)
	}

	var got []widget
	if err := queryutil.ApplySearchAndFilters(db.Model(&widget{}), params).Find(&got).Error; err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Beta" {
		t.Errorf("got %+v, want single Beta", got)
	}
}

func TestBuildListParamsRejectsUnknownIdentifiers(t *testing.T) {
	for _, req := range []listRequest{
		{Sort: "id:asc"},
		{Filters: []string{"id:eq:1"}},
		{Filters: []string{"price:drop:1"}},
	} {
		_, err := queryutil.BuildListParams(req)
		if !errors.Is(err, queryutil.ErrInvalidQuery) {
			t.Errorf("BuildListParams(%+v) error = %v, want ErrInvalidQuery", req, err)
		}
	}
}

func TestPaginationLimitsForDirectCallers(t *testing.T) {
	for _, test := range []struct {
		name  string
		req   listRequest
		field string
	}{
		{"negative page", listRequest{Page: -1}, "page"},
		{"negative size", listRequest{PageSize: -1}, "page_size"},
		{"large size", listRequest{PageSize: 101}, "page_size"},
		{"deep page", listRequest{Page: 1002, PageSize: 100}, "page"},
		{"overflow", listRequest{Page: int(^uint(0) >> 1), PageSize: 100}, "page"},
		{"empty preload allowlist", listRequest{Preload: []string{"Category"}}, "preload"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := queryutil.BuildListParams(test.req)
			var detail *validation.Error
			if !errors.Is(err, queryutil.ErrInvalidQuery) || !errors.As(err, &detail) || detail.Field != test.field {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
	params, err := queryutil.BuildListParams(listRequest{})
	if err != nil || params.Limit != 20 || params.Offset != 0 || params.SortBy != "id" {
		t.Fatalf("unexpected defaults: %+v, %v", params, err)
	}
	params, err = queryutil.BuildListParams(listRequest{Page: 1001, PageSize: 100})
	if err != nil || params.Offset != queryutil.MaxOffset {
		t.Fatalf("unexpected boundary: %+v, %v", params, err)
	}
}

func TestBindRejectsMalformedPagination(t *testing.T) {
	for _, key := range []string{"page", "page_size"} {
		for _, value := range []string{"abc", "0", "-1", strconv.FormatUint(^uint64(0), 10)} {
			t.Run(key+"/"+value, func(t *testing.T) {
				var req listRequest
				if err := queryutil.Bind(url.Values{key: {value}}, &req); !errors.Is(err, queryutil.ErrInvalidQuery) {
					t.Fatalf("error = %v, want invalid query", err)
				}
			})
		}
	}
}

func TestStablePaginationOrdering(t *testing.T) {
	db := setupDB(t)
	if err := db.Model(&widget{}).Where("1 = 1").Update("price", 5).Error; err != nil {
		t.Fatal(err)
	}
	for _, sort := range []string{"", "price:desc"} {
		for page := 1; page <= 3; page++ {
			params, err := queryutil.BuildListParams(listRequest{Page: page, PageSize: 1, Sort: sort})
			if err != nil {
				t.Fatal(err)
			}
			var products []widget
			if err := queryutil.ApplySortPreloadPage(db.Model(&widget{}), params).Find(&products).Error; err != nil {
				t.Fatal(err)
			}
			if len(products) != 1 || products[0].ID != strconv.Itoa(page) {
				t.Fatalf("sort=%q page=%d: unexpected products %+v", sort, page, products)
			}
		}
	}
}

func TestPreloadAllowlistAcceptsOnlyDeclaredRelations(t *testing.T) {
	type relationRequest struct {
		Preload []string `query:"preload,list" preload:"Category"`
	}
	params, err := queryutil.BuildListParams(relationRequest{Preload: []string{"Category"}})
	if err != nil || len(params.Preload) != 1 || params.Preload[0] != "Category" {
		t.Fatalf("unexpected allowed preload: %+v, %v", params, err)
	}
	if _, err := queryutil.BuildListParams(relationRequest{Preload: []string{"Owner"}}); !errors.Is(err, queryutil.ErrInvalidQuery) {
		t.Fatalf("error = %v, want invalid query", err)
	}
}
