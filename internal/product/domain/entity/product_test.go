package entity

import (
	"errors"
	"math"
	"testing"
)

func TestChangeDetailsRejectsInvalidMutation(t *testing.T) {
	valid := ProductDetails{Name: "Widget", Description: "Original", Price: 10, Stock: 5}
	for _, test := range []struct {
		name    string
		details ProductDetails
	}{
		{"empty name", ProductDetails{Price: 1}},
		{"zero price", ProductDetails{Name: "New"}},
		{"negative stock", ProductDetails{Name: "New", Price: 1, Stock: -1}},
		{"nonfinite price", ProductDetails{Name: "New", Price: math.Inf(1)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			product, err := NewProduct("product-1", valid)
			if err != nil {
				t.Fatal(err)
			}
			before := *product
			if err := product.ChangeDetails(test.details); !errors.Is(err, ErrInvalidProduct) {
				t.Fatalf("error = %v, want invalid product", err)
			}
			if *product != before {
				t.Fatal("rejected mutation changed product")
			}
			if _, err := NewProduct("product-1", test.details); !errors.Is(err, ErrInvalidProduct) {
				t.Fatalf("constructor error = %v, want invalid product", err)
			}
		})
	}
}

func TestProductConstructionAndMutation(t *testing.T) {
	details := ProductDetails{Name: "Widget", Price: 1, Stock: 5}
	if _, err := NewProduct(" ", details); !errors.Is(err, ErrInvalidProduct) {
		t.Fatalf("constructor error = %v, want required ID", err)
	}
	product, err := NewProduct("product-1", details)
	if err != nil {
		t.Fatal(err)
	}
	product.Version = 3
	details.Name, details.Stock = "Updated", 0
	if err := product.ChangeDetails(details); err != nil {
		t.Fatal(err)
	}
	if product.Name != "Updated" || product.Stock != 0 || product.ID != "product-1" || product.Version != 3 {
		t.Fatalf("unexpected product: %+v", product)
	}
}
