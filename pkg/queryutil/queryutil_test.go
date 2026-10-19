package queryutil_test

import (
	"errors"
	"net/url"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/boilerplate/microservice-boilerplate/pkg/queryutil"
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
	Sort     string   `query:"sort" sort:"name,price"`
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
