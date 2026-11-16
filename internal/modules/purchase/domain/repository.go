package domain

import "context"

type Repository interface {
	Save(ctx context.Context, order *Order) error
	List(ctx context.Context, tenantID string) ([]*Order, error)
}
