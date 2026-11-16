package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
)

type PermissionRepository struct {
	db *pgxpool.Pool
}

func NewPermissionRepository(db *pgxpool.Pool) *PermissionRepository {
	return &PermissionRepository{db: db}
}

func (r *PermissionRepository) List(ctx context.Context) ([]*domain.Permission, error) {
	rows, err := r.db.Query(ctx, `
		SELECT code, name, module, type, description
		FROM erp.permissions
		ORDER BY module, code`)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	defer rows.Close()

	result := make([]*domain.Permission, 0)
	for rows.Next() {
		permission := &domain.Permission{}
		if err := rows.Scan(&permission.Code, &permission.Name, &permission.Module, &permission.Type, &permission.Description); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		result = append(result, permission)
	}
	return result, rows.Err()
}
