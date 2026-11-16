package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO erp.users (
			id, tenant_id, primary_org_id, username, display_name, email,
			password_hash, status, is_super_admin, created_at, updated_at
		) VALUES (
			$1, $2, NULLIF($3, '')::uuid, $4, $5, NULLIF($6, ''),
			$7, $8, $9, now(), now()
		)`,
		user.ID, user.TenantID, user.OrgID, user.Username, user.DisplayName, user.Email,
		user.PasswordHash, user.Status, user.IsSuperAdmin,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", mapDBError(err))
	}
	return nil
}

func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE erp.users
		SET primary_org_id = NULLIF($3, '')::uuid,
		    display_name = $4,
		    email = NULLIF($5, ''),
		    status = $6,
		    is_super_admin = $7,
		    password_hash = $8,
		    updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		user.TenantID, user.ID, user.OrgID, user.DisplayName, user.Email,
		user.Status, user.IsSuperAdmin, user.PasswordHash,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", mapDBError(err))
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepository) FindByID(ctx context.Context, tenantID, id string) (*domain.User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, COALESCE(primary_org_id::text, ''),
		       username, display_name, COALESCE(email, ''), password_hash,
		       status, is_super_admin, last_login_at, created_at, updated_at
		FROM erp.users
		WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return scanUser(row)
}

func (r *UserRepository) FindByUsername(ctx context.Context, tenantID, username string) (*domain.User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, COALESCE(primary_org_id::text, ''),
		       username, display_name, COALESCE(email, ''), password_hash,
		       status, is_super_admin, last_login_at, created_at, updated_at
		FROM erp.users
		WHERE tenant_id = $1 AND username = $2`, tenantID, username)
	return scanUser(row)
}

func (r *UserRepository) List(ctx context.Context, tenantID, keyword string, offset, limit int) ([]*domain.User, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `
		SELECT count(*)
		FROM erp.users
		WHERE tenant_id = $1
		  AND ($2 = '' OR username ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%' OR COALESCE(email, '') ILIKE '%' || $2 || '%')`,
		tenantID, keyword,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT id::text, tenant_id::text, COALESCE(primary_org_id::text, ''),
		       username, display_name, COALESCE(email, ''), password_hash,
		       status, is_super_admin, last_login_at, created_at, updated_at
		FROM erp.users
		WHERE tenant_id = $1
		  AND ($2 = '' OR username ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%' OR COALESCE(email, '') ILIKE '%' || $2 || '%')
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`, tenantID, keyword, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]*domain.User, 0)
	for rows.Next() {
		user, scanErr := scanUser(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate users: %w", err)
	}
	return users, total, nil
}

func (r *UserRepository) SetRoles(ctx context.Context, tenantID, userID string, roleIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin set user roles: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM erp.user_roles WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID); err != nil {
		return fmt.Errorf("delete user roles: %w", err)
	}

	for _, roleID := range roleIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.user_roles (tenant_id, user_id, role_id)
			VALUES ($1, $2, $3)`, tenantID, userID, roleID); err != nil {
			return fmt.Errorf("insert user role: %w", mapDBError(err))
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit user roles: %w", err)
	}
	return nil
}

func (r *UserRepository) TouchLastLogin(ctx context.Context, tenantID, userID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE erp.users
		SET last_login_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND id = $2`, tenantID, userID)
	if err != nil {
		return fmt.Errorf("touch last login: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepository) CountActiveSuperAdmins(ctx context.Context, tenantID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT count(*)
		FROM erp.users
		WHERE tenant_id = $1 AND is_super_admin = true AND status = 'active'`, tenantID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active super admins: %w", err)
	}
	return count, nil
}

func (r *UserRepository) DataScopeByUserID(ctx context.Context, tenantID, userID, orgID string) (string, error) {
	var dataScope string
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE((
			SELECT data_scope
			FROM erp.org_memberships
			WHERE tenant_id = $1
			  AND user_id = $2
			  AND (NULLIF($3, '')::uuid IS NULL OR org_id = NULLIF($3, '')::uuid)
			ORDER BY created_at
			LIMIT 1
		), 'self')`, tenantID, userID, orgID).Scan(&dataScope)
	if err != nil {
		return "", fmt.Errorf("load user data scope: %w", err)
	}
	if dataScope == "" {
		dataScope = "self"
	}
	return dataScope, nil
}

func (r *UserRepository) GetRoleIDs(ctx context.Context, tenantID, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT role_id::text FROM erp.user_roles
		WHERE tenant_id = $1 AND user_id = $2
		ORDER BY role_id`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("list user roles: %w", err)
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var roleID string
		if err := rows.Scan(&roleID); err != nil {
			return nil, fmt.Errorf("scan user role: %w", err)
		}
		result = append(result, roleID)
	}
	return result, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*domain.User, error) {
	var user domain.User
	var lastLoginAt *time.Time

	err := row.Scan(
		&user.ID, &user.TenantID, &user.OrgID,
		&user.Username, &user.DisplayName, &user.Email, &user.PasswordHash,
		&user.Status, &user.IsSuperAdmin, &lastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	user.LastLoginAt = lastLoginAt
	return &user, nil
}
