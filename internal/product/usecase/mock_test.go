package usecase_test

import (
	"context"
	"time"

	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/pkg/event"
)

type mockProductRepository struct {
	listProducts []entity.Product
	listTotal    int64
	listRequest  request.ListProductsRequest
	products     map[string]*entity.Product
	createErr    error
	findErr      error
	updateErr    error
	deleteErr    error
	createCalls  int
	findCalls    int
	updateCalls  int
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
	m.listRequest = params
	if m.findErr != nil {
		return nil, 0, m.findErr
	}
	return append([]entity.Product(nil), m.listProducts...), m.listTotal, nil
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
