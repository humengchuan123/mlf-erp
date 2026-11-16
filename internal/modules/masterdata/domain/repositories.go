package domain

import (
	"context"

	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type CrudRepository[T Entity] interface {
	Create(ctx context.Context, entity T) error
	FindByID(ctx context.Context, filter scope.Filter, tenantID, id string) (T, error)
	List(ctx context.Context, filter scope.Filter, tenantID, keyword string, offset, limit int) ([]T, int, error)
	Update(ctx context.Context, filter scope.Filter, entity T) error
	Disable(ctx context.Context, filter scope.Filter, tenantID, id, userID string) error
	Enable(ctx context.Context, filter scope.Filter, tenantID, id, userID string) error
}
