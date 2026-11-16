package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
)

type RoleRepository struct {
	db *pgxpool.Pool
}

func NewRoleRepository(db *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) Create(ctx context.Context, role *domain.Role) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO erp.roles (id, tenant_id, code, name, is_system, created_at)
		VALUES ($1, $2, $3, $4, $5, now())`,
		role.ID, role.TenantID, role.Code, role.Name, role.IsSystem,
	)
	if err != nil {
		return fmt.Errorf("create role: %w", err)
	}
	return nil
}

func (r *RoleRepository) Update(ctx context.Context, role *domain.Role) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE erp.roles
		SET code = $3, name = $4, is_system = $5
		WHERE tenant_id = $1 AND id = $2`,
		role.TenantID, role.ID, role.Code, role.Name, role.IsSystem,
	)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *RoleRepository) Delete(ctx context.Context, tenantID, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM erp.roles WHERE tenant_id = $1 AND id = $2 AND is_system = false`, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *RoleRepository) FindByID(ctx context.Context, tenantID, id string) (*domain.Role, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, code, name, is_system, created_at
		FROM erp.roles
		WHERE tenant_id = $1 AND id = $2`, tenantID, id)

	role, err := scanRole(row)
	if err != nil {
		return nil, err
	}
	permissions, err := r.permissionCodes(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	role.Permissions = permissions
	return role, nil
}

func (r *RoleRepository) List(ctx context.Context, tenantID string) ([]*domain.Role, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, tenant_id::text, code, name, is_system, created_at
		FROM erp.roles
		WHERE tenant_id = $1
		ORDER BY is_system DESC, code`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()

	roles := make([]*domain.Role, 0)
	for rows.Next() {
		role, scanErr := scanRole(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		permissions, permErr := r.permissionCodes(ctx, tenantID, role.ID)
		if permErr != nil {
			return nil, permErr
		}
		role.Permissions = permissions
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *RoleRepository) SetPermissions(ctx context.Context, tenantID, roleID string, permissionCodes []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin set role permissions: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM erp.role_permissions WHERE tenant_id = $1 AND role_id = $2`, tenantID, roleID); err != nil {
		return fmt.Errorf("delete role permissions: %w", err)
	}

	for _, code := range permissionCodes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
			VALUES ($1, $2, $3)`, tenantID, roleID, code); err != nil {
			return fmt.Errorf("insert role permission: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit role permissions: %w", err)
	}
	return nil
}

func (r *RoleRepository) PermissionCodesByUserID(ctx context.Context, tenantID, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT rp.permission_code
		FROM erp.user_roles ur
		JOIN erp.role_permissions rp
		  ON rp.tenant_id = ur.tenant_id AND rp.role_id = ur.role_id
		WHERE ur.tenant_id = $1 AND ur.user_id = $2
		ORDER BY rp.permission_code`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("list user permissions: %w", err)
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan user permission: %w", err)
		}
		result = append(result, code)
	}
	return result, rows.Err()
}

func (r *RoleRepository) RoleCodesByUserID(ctx context.Context, tenantID, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT r.code
		FROM erp.user_roles ur
		JOIN erp.roles r ON r.tenant_id = ur.tenant_id AND r.id = ur.role_id
		WHERE ur.tenant_id = $1 AND ur.user_id = $2
		ORDER BY r.code`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("list user role codes: %w", err)
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan user role code: %w", err)
		}
		result = append(result, code)
	}
	return result, rows.Err()
}

func (r *RoleRepository) permissionCodes(ctx context.Context, tenantID, roleID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT permission_code
		FROM erp.role_permissions
		WHERE tenant_id = $1 AND role_id = $2
		ORDER BY permission_code`, tenantID, roleID)
	if err != nil {
		return nil, fmt.Errorf("list role permissions: %w", err)
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan role permission: %w", err)
		}
		result = append(result, code)
	}
	return result, rows.Err()
}

func scanRole(row rowScanner) (*domain.Role, error) {
	var role domain.Role
	err := row.Scan(&role.ID, &role.TenantID, &role.Code, &role.Name, &role.IsSystem, &role.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan role: %w", err)
	}
	return &role, nil
}
