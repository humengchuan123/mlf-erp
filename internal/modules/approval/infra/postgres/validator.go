package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Validator struct {
	db *pgxpool.Pool
}

func NewValidator(db *pgxpool.Pool) *Validator {
	return &Validator{db: db}
}

func (v *Validator) RoleExists(ctx context.Context, tenantID, roleID string) (bool, error) {
	if _, err := uuid.Parse(roleID); err != nil {
		return false, nil
	}

	var exists bool
	if err := v.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM erp.roles
			WHERE tenant_id = $1 AND id = $2
		)`, tenantID, roleID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check approval role exists: %w", err)
	}
	return exists, nil
}

func (v *Validator) UserExists(ctx context.Context, tenantID, userID string) (bool, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return false, nil
	}

	var exists bool
	if err := v.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM erp.users
			WHERE tenant_id = $1 AND id = $2
		)`, tenantID, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check approval user exists: %w", err)
	}
	return exists, nil
}
