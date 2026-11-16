package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/inventory/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type QueryRepository struct {
	db *pgxpool.Pool
}

func NewQueryRepository(db *pgxpool.Pool) *QueryRepository {
	return &QueryRepository{db: db}
}

func (r *QueryRepository) ListBalances(
	ctx context.Context,
	filter scope.Filter,
	tenantID, warehouseID, materialID, keyword string,
	offset, limit int,
) ([]*domain.Balance, int, error) {
	args := []any{tenantID}
	where := "b.tenant_id = $1"
	where, args = appendInventoryFilters(where, args, "b", filter, warehouseID, materialID, keyword)

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM erp.inventory_balances b
		JOIN erp.warehouses w ON w.tenant_id = b.tenant_id AND w.id = b.warehouse_id
		JOIN erp.materials m ON m.tenant_id = b.tenant_id AND m.id = b.material_id
		WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count inventory balances: %w", err)
	}

	query := `SELECT b.tenant_id::text, b.org_id::text, b.warehouse_id::text, w.name,
	                 b.material_id::text, m.name, b.on_hand::text, b.reserved::text,
	                 b.available::text, b.updated_at
	          FROM erp.inventory_balances b
	          JOIN erp.warehouses w ON w.tenant_id = b.tenant_id AND w.id = b.warehouse_id
	          JOIN erp.materials m ON m.tenant_id = b.tenant_id AND m.id = b.material_id
	          WHERE ` + where + fmt.Sprintf(` ORDER BY w.code, m.code LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list inventory balances: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Balance, 0)
	for rows.Next() {
		item := &domain.Balance{}
		var onHand, reserved, available string
		if err := rows.Scan(
			&item.TenantID, &item.OrgID, &item.WarehouseID, &item.WarehouseName,
			&item.MaterialID, &item.MaterialName, &onHand, &reserved, &available, &item.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan inventory balance: %w", err)
		}
		item.OnHand = decimal.RequireFromString(onHand)
		item.Reserved = decimal.RequireFromString(reserved)
		item.Available = decimal.RequireFromString(available)
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *QueryRepository) ListTransactions(
	ctx context.Context,
	filter scope.Filter,
	tenantID, warehouseID, materialID, keyword string,
	offset, limit int,
) ([]*domain.Transaction, int, error) {
	args := []any{tenantID}
	where := "t.tenant_id = $1"
	where, args = appendInventoryFilters(where, args, "t", filter, warehouseID, materialID, keyword)

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM erp.inventory_transactions t
		JOIN erp.warehouses w ON w.tenant_id = t.tenant_id AND w.id = t.warehouse_id
		JOIN erp.materials m ON m.tenant_id = t.tenant_id AND m.id = t.material_id
		WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count inventory transactions: %w", err)
	}

	query := `SELECT t.id::text, t.tenant_id::text, t.org_id::text, t.warehouse_id::text, w.name,
	                 t.material_id::text, m.name, t.direction, t.biz_type, t.biz_id::text, t.biz_code,
	                 t.quantity::text, t.unit_cost::text, t.amount::text, t.occurred_at, t.created_by::text
	          FROM erp.inventory_transactions t
	          JOIN erp.warehouses w ON w.tenant_id = t.tenant_id AND w.id = t.warehouse_id
	          JOIN erp.materials m ON m.tenant_id = t.tenant_id AND m.id = t.material_id
	          WHERE ` + where + fmt.Sprintf(` ORDER BY t.occurred_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list inventory transactions: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Transaction, 0)
	for rows.Next() {
		item := &domain.Transaction{}
		var quantity, unitCost, amount string
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.OrgID, &item.WarehouseID, &item.WarehouseName,
			&item.MaterialID, &item.MaterialName, &item.Direction, &item.BizType,
			&item.BizID, &item.BizCode, &quantity, &unitCost, &amount, &item.OccurredAt, &item.CreatedBy,
		); err != nil {
			return nil, 0, fmt.Errorf("scan inventory transaction: %w", err)
		}
		item.Quantity = decimal.RequireFromString(quantity)
		item.UnitCost = decimal.RequireFromString(unitCost)
		item.Amount = decimal.RequireFromString(amount)
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func appendInventoryFilters(
	where string,
	args []any,
	alias string,
	filter scope.Filter,
	warehouseID, materialID, keyword string,
) (string, []any) {
	if warehouseID != "" {
		args = append(args, warehouseID)
		where += fmt.Sprintf(" AND %s.warehouse_id = $%d", alias, len(args))
	}
	if materialID != "" {
		args = append(args, materialID)
		where += fmt.Sprintf(" AND %s.material_id = $%d", alias, len(args))
	}
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		where += fmt.Sprintf(" AND (w.name ILIKE $%d OR m.name ILIKE $%d OR w.code ILIKE $%d OR m.code ILIKE $%d)", len(args), len(args), len(args), len(args))
	}

	switch filter.DataScope {
	case "all":
		return where, args
	case "self":
		if filter.OrgID == "" {
			return where + " AND 1 = 0", args
		}
		args = append(args, filter.OrgID)
		where += fmt.Sprintf(" AND %s.org_id = $%d", alias, len(args))
		return where, args
	case "dept":
		if filter.OrgPath == "" {
			return where + " AND 1 = 0", args
		}
		args = append(args, filter.OrgPath)
		where += fmt.Sprintf(
			" AND EXISTS (SELECT 1 FROM erp.organizations o WHERE o.tenant_id = %s.tenant_id AND o.id = %s.org_id AND (o.path = $%d OR o.path LIKE $%d || '/%%'))",
			alias, alias, len(args), len(args),
		)
		return where, args
	case "company":
		if filter.CompanyPath == "" {
			return where + " AND 1 = 0", args
		}
		args = append(args, filter.CompanyPath)
		where += fmt.Sprintf(
			" AND EXISTS (SELECT 1 FROM erp.organizations o WHERE o.tenant_id = %s.tenant_id AND o.id = %s.org_id AND (o.path = $%d OR o.path LIKE $%d || '/%%'))",
			alias, alias, len(args), len(args),
		)
		return where, args
	default:
		return where + " AND 1 = 0", args
	}
}
