package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type Repository[T domain.Entity] struct {
	db *pgxpool.Pool
}

func NewRepository[T domain.Entity](db *pgxpool.Pool) *Repository[T] {
	return &Repository[T]{db: db}
}

func (r *Repository[T]) Create(ctx context.Context, entity T) error {
	columns := entity.InsertColumns()
	values := entity.InsertValues()
	placeholders := make([]string, 0, len(columns))
	for i := range columns {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		entity.TableName(),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)
	if _, err := r.db.Exec(ctx, query, values...); err != nil {
		return fmt.Errorf("create %s: %w", entity.TableName(), mapDBError(err))
	}
	return nil
}

func (r *Repository[T]) FindByID(ctx context.Context, filter scope.Filter, tenantID, id string) (T, error) {
	var zero T

	args := []any{tenantID, id}
	where := "p.tenant_id = $1 AND p.id = $2"
	if entityScoped(zero) {
		appendScope(&where, &args, filter)
	}

	query := fmt.Sprintf(
		"SELECT %s FROM %s AS p WHERE %s LIMIT 1",
		strings.Join(zero.SelectColumns(), ", "),
		zero.TableName(),
		where,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return zero, fmt.Errorf("find %s: %w", zero.TableName(), err)
	}
	defer rows.Close()

	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		return zero, fmt.Errorf("scan %s: %w", zero.TableName(), err)
	}
	if len(items) == 0 {
		return zero, domain.ErrNotFound
	}
	return items[0], nil
}

func (r *Repository[T]) List(
	ctx context.Context,
	filter scope.Filter,
	tenantID, keyword string,
	offset, limit int,
) ([]T, int, error) {
	var zero T

	args := []any{tenantID}
	where := "p.tenant_id = $1"

	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		searchParts := make([]string, 0)
		for _, column := range zero.SearchColumns() {
			searchParts = append(searchParts, fmt.Sprintf("p.%s ILIKE $%d", column, len(args)))
		}
		if len(searchParts) > 0 {
			where += " AND (" + strings.Join(searchParts, " OR ") + ")"
		}
	}
	if entityScoped(zero) {
		appendScope(&where, &args, filter)
	}

	countQuery := fmt.Sprintf("SELECT count(*) FROM %s AS p WHERE %s", zero.TableName(), where)
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count %s: %w", zero.TableName(), err)
	}

	dataQuery := fmt.Sprintf(
		"SELECT %s FROM %s AS p WHERE %s ORDER BY p.created_at DESC, p.code LIMIT $%d OFFSET $%d",
		strings.Join(zero.SelectColumns(), ", "),
		zero.TableName(),
		where,
		len(args)+1,
		len(args)+2,
	)
	dataArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.db.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list %s: %w", zero.TableName(), err)
	}
	defer rows.Close()

	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		return nil, 0, fmt.Errorf("scan %s: %w", zero.TableName(), err)
	}
	return items, total, nil
}

func (r *Repository[T]) Update(ctx context.Context, filter scope.Filter, entity T) error {
	columns := entity.UpdateColumns()
	values := entity.UpdateValues()

	args := []any{entity.GetTenantID(), entity.GetID()}
	setClauses := make([]string, 0, len(columns)+1)
	for i, column := range columns {
		args = append(args, values[i])
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	setClauses = append(setClauses, "updated_at = now()")

	where := "p.tenant_id = $1 AND p.id = $2"
	if entity.Scoped() {
		appendScope(&where, &args, filter)
	}

	query := fmt.Sprintf(
		"UPDATE %s AS p SET %s WHERE %s",
		entity.TableName(),
		strings.Join(setClauses, ", "),
		where,
	)
	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update %s: %w", entity.TableName(), mapDBError(err))
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository[T]) Disable(ctx context.Context, filter scope.Filter, tenantID, id, userID string) error {
	var zero T

	args := []any{tenantID, id}
	setClause := "status = 'disabled', updated_at = now()"
	where := "p.tenant_id = $1 AND p.id = $2"

	if zero.Scoped() {
		args = append(args, userID)
		setClause = "status = 'disabled', updated_by = $3, updated_at = now()"
		appendScope(&where, &args, filter)
	}

	query := fmt.Sprintf("UPDATE %s AS p SET %s WHERE %s", zero.TableName(), setClause, where)
	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("disable %s: %w", zero.TableName(), err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func entityScoped[T domain.Entity](entity T) bool {
	return entity.Scoped()
}

func appendScope(where *string, args *[]any, filter scope.Filter) {
	if filter.DataScope == "" || filter.DataScope == "all" {
		return
	}

	switch filter.DataScope {
	case "self":
		*args = append(*args, filter.UserID)
		*where += fmt.Sprintf(" AND p.created_by = $%d", len(*args))
	case "dept":
		*args = append(*args, filter.OrgPath)
		*where += fmt.Sprintf(
			" AND EXISTS (SELECT 1 FROM erp.organizations o WHERE o.tenant_id = p.tenant_id AND o.id = p.org_id AND (o.path = $%d OR o.path LIKE $%d || '/%%'))",
			len(*args), len(*args),
		)
	case "company":
		*args = append(*args, filter.CompanyPath)
		*where += fmt.Sprintf(
			" AND EXISTS (SELECT 1 FROM erp.organizations o WHERE o.tenant_id = p.tenant_id AND o.id = p.org_id AND (o.path = $%d OR o.path LIKE $%d || '/%%'))",
			len(*args), len(*args),
		)
	default:
		*args = append(*args, filter.UserID)
		*where += fmt.Sprintf(" AND p.created_by = $%d", len(*args))
	}
}

func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "23503":
			return domain.ErrConflict
		case "22P02", "23502":
			return domain.ErrInvalidInput
		}
	}
	return err
}
