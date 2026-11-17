package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
)

type AccountRepository struct {
	db *pgxpool.Pool
}

func NewAccountRepository(db *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{db: db}
}

func (r *AccountRepository) List(ctx context.Context, tenantID string) ([]*domain.Account, error) {
	rows, err := r.db.Query(ctx, accountSelectSQL+` WHERE tenant_id = $1 ORDER BY code`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Account, 0)
	for rows.Next() {
		item, scanErr := scanAccount(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *AccountRepository) Get(ctx context.Context, tenantID, id string) (*domain.Account, error) {
	row := r.db.QueryRow(ctx, accountSelectSQL+` WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return scanAccount(row)
}

func (r *AccountRepository) Create(ctx context.Context, account *domain.Account) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create account: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.gl_accounts (
			id, tenant_id, code, name, type, direction, parent_id, level, is_leaf, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		account.ID, account.TenantID, account.Code, account.Name, account.Type,
		account.Direction, nullableUUID(account.ParentID), account.Level, account.IsLeaf, account.Status,
	); err != nil {
		return fmt.Errorf("insert account: %w", mapDBError(err))
	}
	if account.ParentID != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE erp.gl_accounts SET is_leaf = false, updated_at = now()
			WHERE tenant_id = $1 AND id = $2`, account.TenantID, account.ParentID); err != nil {
			return fmt.Errorf("mark parent account: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create account: %w", err)
	}
	return nil
}

func (r *AccountRepository) Update(ctx context.Context, account *domain.Account) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update account: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE erp.gl_accounts
		SET code = $3, name = $4, type = $5, direction = $6,
		    parent_id = $7, level = $8, status = $9, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		account.TenantID, account.ID, account.Code, account.Name, account.Type,
		account.Direction, nullableUUID(account.ParentID), account.Level, account.Status,
	)
	if err != nil {
		return fmt.Errorf("update account: %w", mapDBError(err))
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if account.ParentID != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE erp.gl_accounts SET is_leaf = false, updated_at = now()
			WHERE tenant_id = $1 AND id = $2`, account.TenantID, account.ParentID); err != nil {
			return fmt.Errorf("mark parent account: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit update account: %w", err)
	}
	return nil
}

func (r *AccountRepository) InUse(ctx context.Context, tenantID, id string) (bool, error) {
	var inUse bool
	if err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM erp.gl_voucher_lines
			WHERE tenant_id = $1 AND account_id = $2
		) OR EXISTS (
			SELECT 1
			FROM erp.gl_account_mappings m
			JOIN erp.gl_accounts a ON a.tenant_id = m.tenant_id AND a.code = m.account_code
			WHERE m.tenant_id = $1 AND a.id = $2
		)`, tenantID, id).Scan(&inUse); err != nil {
		return false, fmt.Errorf("check account usage: %w", err)
	}
	return inUse, nil
}

const accountSelectSQL = `
	SELECT id::text, tenant_id::text, code, name, type, direction,
	       COALESCE(parent_id::text, ''), level, is_leaf, status, created_at, updated_at
	FROM erp.gl_accounts`

func scanAccount(row rowScanner) (*domain.Account, error) {
	var item domain.Account
	err := row.Scan(
		&item.ID, &item.TenantID, &item.Code, &item.Name, &item.Type, &item.Direction,
		&item.ParentID, &item.Level, &item.IsLeaf, &item.Status, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan account: %w", err)
	}
	return &item, nil
}

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}
