package usecase_test

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/pkg/event"
	"github.com/boilerplate/microservice-boilerplate/pkg/queryutil"
)

type mockProductRepository struct {
	products    map[string]*entity.Product
	createErr   error
	findErr     error
	updateErr   error
	deleteErr   error
	createCalls int
	findCalls   int
	updateCalls int
}

func newMockRepo() *mockProductRepository {
	return &mockProductRepository{products: make(map[string]*entity.Product)}
}

func (m *mockProductRepository) Create(_ context.Context, p *entity.Product) error {
	m.createCalls++
	if m.createErr != nil {
		return m.createErr
	}
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	stored := *p
	m.products[p.ID] = &stored
	return nil
}

func (m *mockProductRepository) FindByID(_ context.Context, id string) (*entity.Product, error) {
	m.findCalls++
	if m.findErr != nil {
		return nil, m.findErr
	}
	p, ok := m.products[id]
	if !ok {
		return nil, entity.ErrProductNotFound
	}
	product := *p
	return &product, nil
}

func (m *mockProductRepository) FindAllPaginated(_ context.Context, params request.ListProductsRequest) ([]entity.Product, int64, error) {
	if m.findErr != nil {
		return nil, 0, m.findErr
	}
	query, err := queryutil.BuildListParams(params)
	if err != nil {
		return nil, 0, err
	}
	all := make([]entity.Product, 0, len(m.products))
	for _, p := range m.products {
		if query.Search != "" {
			s := strings.ToLower(query.Search)
			if !strings.Contains(strings.ToLower(p.Name), s) && !strings.Contains(strings.ToLower(p.Description), s) {
				continue
			}
		}
		if !matchesFilters(p, query.Filters) {
			continue
		}
		all = append(all, *p)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	total := int64(len(all))
	if query.Offset >= len(all) {
		return []entity.Product{}, total, nil
	}
	end := query.Offset + query.Limit
	if end > len(all) {
		end = len(all)
	}
	return all[query.Offset:end], total, nil
}

func matchesFilters(p *entity.Product, filters []queryutil.Filter) bool {
	for _, filter := range filters {
		var actual any
		switch filter.Column {
		case "name":
			actual = p.Name
		case "price":
			actual = p.Price
		case "stock":
			actual = p.Stock
		default:
			return false
		}
		if !matchesFilter(actual, filter.Op, filter.Value) {
			return false
		}
	}
	return true
}

func matchesFilter(actual any, op string, want any) bool {
	switch value := actual.(type) {
	case string:
		needle := want.(string)
		switch op {
		case "=":
			return value == needle
		case "LIKE":
			return strings.Contains(value, needle)
		}
	case float64:
		needle := want.(float64)
		switch op {
		case "=":
			return value == needle
		case ">=":
			return value >= needle
		case "<=":
			return value <= needle
		}
	case int:
		needle := want.(int)
		switch op {
		case "=":
			return value == needle
		case ">=":
			return value >= needle
		case "<=":
			return value <= needle
		}
	}
	return false
}

func (m *mockProductRepository) Update(_ context.Context, p *entity.Product) error {
	m.updateCalls++
	if m.updateErr != nil {
		return m.updateErr
	}
	stored, ok := m.products[p.ID]
	if !ok {
		return entity.ErrProductNotFound
	}
	if stored.Version != p.Version {
		return entity.ErrProductConflict
	}
	updated := *stored
	updated.Name = p.Name
	updated.Description = p.Description
	updated.Price = p.Price
	updated.Stock = p.Stock
	updated.Version++
	updated.UpdatedAt = time.Now().UTC()
	m.products[p.ID] = &updated
	p.Version = updated.Version
	p.UpdatedAt = updated.UpdatedAt
	return nil
}

func (m *mockProductRepository) Delete(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.products[id]; !ok {
		return entity.ErrProductNotFound
	}
	delete(m.products, id)
	return nil
}

type mockProductCache struct {
	product   *entity.Product
	getErr    error
	setErr    error
	deleteErr error
}

func (m *mockProductCache) Get(_ context.Context, _ string) (*entity.Product, string, error) {
	return m.product, "generation", m.getErr
}

func (m *mockProductCache) Set(_ context.Context, _ *entity.Product, _ string) error {
	return m.setErr
}

func (m *mockProductCache) Delete(_ context.Context, _ string) error {
	return m.deleteErr
}

type mockPublisher struct {
	published []event.Event
	err       error
}

func (m *mockPublisher) Publish(_ context.Context, e event.Event) error {
	m.published = append(m.published, e)
	return m.err
}
