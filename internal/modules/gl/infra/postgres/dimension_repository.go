package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
)

type DimensionRepository struct {
	db *pgxpool.Pool
}

func NewDimensionRepository(db *pgxpool.Pool) *DimensionRepository {
	return &DimensionRepository{db: db}
}

func (r *DimensionRepository) List(ctx context.Context, tenantID string) (*domain.Dimensions, error) {
	partners, err := r.list(ctx, tenantID, `SELECT id::text, code, name FROM erp.partners
		WHERE tenant_id = $1 AND status = 'active' ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("list partner dimensions: %w", err)
	}
	departments, err := r.list(ctx, tenantID, `SELECT id::text, code, name FROM erp.organizations
		WHERE tenant_id = $1 AND status = 'active' AND type = 'dept' ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("list department dimensions: %w", err)
	}
	return &domain.Dimensions{Partners: partners, Departments: departments}, nil
}

func (r *DimensionRepository) list(ctx context.Context, tenantID, query string) ([]*domain.Dimension, error) {
	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*domain.Dimension, 0)
	for rows.Next() {
		item := &domain.Dimension{}
		if err := rows.Scan(&item.ID, &item.Code, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
