package domain

import (
	"context"
	"time"

	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	UpdateDraft(ctx context.Context, filter scope.Filter, order *Order) error
	Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*Order, error)
	List(ctx context.Context, filter scope.Filter, tenantID, status, keyword string, offset, limit int) ([]*Order, int, error)
	UpdateStatus(ctx context.Context, filter scope.Filter, tenantID, id string, from, to Status, userID string) (bool, error)
	Ship(ctx context.Context, filter scope.Filter, shipment *Shipment) error
	ListShipments(ctx context.Context, filter scope.Filter, tenantID, orderID string, offset, limit int) ([]*Shipment, int, error)
}

type ReturnRepository interface {
	Create(ctx context.Context, filter scope.Filter, tenantID, userID string, params ReturnCreateParams) (*SalesReturn, error)
	List(ctx context.Context, filter scope.Filter, tenantID, shipmentID, customerID, keyword string, startDate, endDate time.Time, offset, limit int) ([]*SalesReturn, int, error)
	Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*SalesReturn, error)
}
