package infra

import (
	"context"
	"sort"
	"sync"

	"github.com/mlf-erp/mlf-erp/internal/modules/purchase/domain"
)

type MemoryRepository struct {
	mu     sync.RWMutex
	orders map[string]*domain.Order
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		orders: make(map[string]*domain.Order),
	}
}

func (r *MemoryRepository) Save(_ context.Context, order *domain.Order) error {
	copied := *order

	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[order.ID] = &copied

	return nil
}

func (r *MemoryRepository) List(_ context.Context, tenantID string) ([]*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domain.Order, 0, len(r.orders))
	for _, order := range r.orders {
		if order.TenantID != tenantID {
			continue
		}
		copied := *order
		result = append(result, &copied)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}
