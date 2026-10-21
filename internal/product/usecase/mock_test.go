package usecase_test

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	domainEvent "github.com/boilerplate/microservice-boilerplate/internal/product/domain/event"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/pkg/queryutil"
)

type mockProductRepository struct {
	products  map[string]*entity.Product
	createErr error
	findErr   error
	updateErr error
	deleteErr error
}

func newMockRepo() *mockProductRepository {
	return &mockProductRepository{products: make(map[string]*entity.Product)}
}

func (m *mockProductRepository) Create(_ context.Context, p *entity.Product) error {
	if m.createErr != nil {
		return m.createErr
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = p.CreatedAt
	m.products[p.ID] = p
	return nil
}

func (m *mockProductRepository) FindByID(_ context.Context, id string) (*entity.Product, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	p, ok := m.products[id]
	if !ok {
		return nil, entity.ErrProductNotFound
	}
	return p, nil
}

func (m *mockProductRepository) FindByIDForUpdate(ctx context.Context, id string) (*entity.Product, error) {
	return m.FindByID(ctx, id)
}

func (m *mockProductRepository) FindAll(_ context.Context) ([]entity.Product, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	out := make([]entity.Product, 0, len(m.products))
	for _, p := range m.products {
		out = append(out, *p)
	}
	return out, nil
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
	if m.updateErr != nil {
		return m.updateErr
	}
	p.UpdatedAt = time.Now()
	m.products[p.ID] = p
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

type mockProductCache struct{}

func (m *mockProductCache) Get(_ context.Context, _ string) (*entity.Product, string, error) {
	return nil, "generation", nil
}

func (m *mockProductCache) Set(_ context.Context, _ *entity.Product, _ time.Duration, _ string) error {
	return nil
}

func (m *mockProductCache) Delete(_ context.Context, _ string) error {
	return nil
}

type mockPublisher struct {
	published []domainEvent.Event
}

func (m *mockPublisher) Publish(_ context.Context, e domainEvent.Event) error {
	m.published = append(m.published, e)
	return nil
}
