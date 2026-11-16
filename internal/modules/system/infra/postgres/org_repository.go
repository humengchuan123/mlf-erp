package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
)

type OrgRepository struct {
	db *pgxpool.Pool
}

func NewOrgRepository(db *pgxpool.Pool) *OrgRepository {
	return &OrgRepository{db: db}
}

func (r *OrgRepository) Create(ctx context.Context, org *domain.Org) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO erp.organizations (
			id, tenant_id, parent_id, type, code, name, path, status, created_at, updated_at
		) VALUES (
			$1, $2, NULLIF($3, '')::uuid, $4, $5, $6, $7, $8, now(), now()
		)`,
		org.ID, org.TenantID, org.ParentID, org.Type, org.Code, org.Name, org.Path, org.Status,
	)
	if err != nil {
		return fmt.Errorf("create org: %w", err)
	}
	return nil
}

func (r *OrgRepository) Update(ctx context.Context, org *domain.Org) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE erp.organizations
		SET parent_id = NULLIF($3, '')::uuid,
		    type = $4, code = $5, name = $6, path = $7, status = $8,
		    updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		org.TenantID, org.ID, org.ParentID, org.Type, org.Code, org.Name, org.Path, org.Status,
	)
	if err != nil {
		return fmt.Errorf("update org: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *OrgRepository) Delete(ctx context.Context, tenantID, id string) error {
	var children int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM erp.organizations WHERE tenant_id = $1 AND parent_id = $2`, tenantID, id).Scan(&children); err != nil {
		return fmt.Errorf("count child orgs: %w", err)
	}
	if children > 0 {
		return domain.ErrOrgHasChildren
	}

	tag, err := r.db.Exec(ctx, `DELETE FROM erp.organizations WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete org: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *OrgRepository) FindByID(ctx context.Context, tenantID, id string) (*domain.Org, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, COALESCE(parent_id::text, ''),
		       type, code, name, path, status, created_at, updated_at
		FROM erp.organizations
		WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return scanOrg(row)
}

func (r *OrgRepository) List(ctx context.Context, tenantID string) ([]*domain.Org, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, tenant_id::text, COALESCE(parent_id::text, ''),
		       type, code, name, path, status, created_at, updated_at
		FROM erp.organizations
		WHERE tenant_id = $1
		ORDER BY path, code`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list orgs: %w", err)
	}
	defer rows.Close()

	result := make([]*domain.Org, 0)
	for rows.Next() {
		org, scanErr := scanOrg(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, org)
	}
	return result, rows.Err()
}

func scanOrg(row rowScanner) (*domain.Org, error) {
	var org domain.Org
	err := row.Scan(
		&org.ID, &org.TenantID, &org.ParentID,
		&org.Type, &org.Code, &org.Name, &org.Path, &org.Status, &org.CreatedAt, &org.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan org: %w", err)
	}
	return &org, nil
}
