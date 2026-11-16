package domain

import (
	"context"

	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type QueryRepository interface {
	ListBalances(ctx context.Context, filter scope.Filter, tenantID, warehouseID, materialID, keyword string, offset, limit int) ([]*Balance, int, error)
	ListTransactions(ctx context.Context, filter scope.Filter, tenantID, warehouseID, materialID, keyword string, offset, limit int) ([]*Transaction, int, error)
}
