package domain

import (
	"context"

	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	UpdateDraft(ctx context.Context, filter scope.Filter, order *Order) error
	Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*Order, error)
	List(ctx context.Context, filter scope.Filter, tenantID, status, keyword string, offset, limit int) ([]*Order, int, error)
	UpdateStatus(ctx context.Context, tenantID, id string, from, to Status, userID string) (bool, error)
	Receive(ctx context.Context, filter scope.Filter, receipt *Receipt) error
	ListReceipts(ctx context.Context, filter scope.Filter, tenantID, orderID string, offset, limit int) ([]*Receipt, int, error)
}
