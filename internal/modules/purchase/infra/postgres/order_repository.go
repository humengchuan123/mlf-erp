package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/purchase/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, order *domain.Order) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create order: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.purchase_orders (
			id, tenant_id, org_id, code, supplier_id, warehouse_id, order_date,
			currency, status, total_amount, remark, created_by, updated_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		order.ID, order.TenantID, order.OrgID, order.Code, order.SupplierID, order.WarehouseID,
		order.OrderDate, order.Currency, string(order.Status), order.TotalAmount.String(),
		order.Remark, order.CreatedBy, order.UpdatedBy,
	); err != nil {
		return fmt.Errorf("insert order: %w", mapDBError(err))
	}
	if err := insertLines(ctx, tx, order); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create order: %w", err)
	}
	return nil
}

func (r *Repository) UpdateDraft(ctx context.Context, filter scope.Filter, order *domain.Order) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update order: %w", err)
	}
	defer tx.Rollback(ctx)

	args := []any{
		order.TenantID, order.ID, order.SupplierID, order.WarehouseID, order.OrderDate,
		order.Currency, order.TotalAmount.String(), order.Remark, order.UpdatedBy,
	}
	where := "o.tenant_id = $1 AND o.id = $2 AND o.status = 'draft'"
	appendScope(&where, &args, "o", filter)

	tag, err := tx.Exec(ctx, `
		UPDATE erp.purchase_orders AS o
		SET supplier_id = $3, warehouse_id = $4, order_date = $5,
		    currency = $6, total_amount = $7, remark = $8, updated_by = $9, updated_at = now()
		WHERE `+where, args...)
	if err != nil {
		return fmt.Errorf("update order: %w", mapDBError(err))
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM erp.purchase_order_lines WHERE tenant_id = $1 AND order_id = $2`, order.TenantID, order.ID); err != nil {
		return fmt.Errorf("delete order lines: %w", err)
	}
	if err := insertLines(ctx, tx, order); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit update order: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*domain.Order, error) {
	args := []any{tenantID, id}
	where := "o.tenant_id = $1 AND o.id = $2"
	appendScope(&where, &args, "o", filter)

	row := r.db.QueryRow(ctx, orderSelectSQL+` WHERE `+where, args...)
	order, err := scanOrderHeader(row)
	if err != nil {
		return nil, err
	}
	lines, err := loadOrderLines(ctx, r.db, tenantID, id)
	if err != nil {
		return nil, err
	}
	order.Lines = lines
	return order, nil
}

func (r *Repository) List(
	ctx context.Context,
	filter scope.Filter,
	tenantID, status, keyword string,
	offset, limit int,
) ([]*domain.Order, int, error) {
	args := []any{tenantID}
	where := "o.tenant_id = $1"

	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND o.status = $%d", len(args))
	}
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		index := len(args)
		where += fmt.Sprintf(" AND (o.code ILIKE $%d OR p.name ILIKE $%d OR w.name ILIKE $%d)", index, index, index)
	}
	appendScope(&where, &args, "o", filter)

	countQuery := `SELECT count(*) FROM erp.purchase_orders o
		LEFT JOIN erp.partners p ON p.tenant_id = o.tenant_id AND p.id = o.supplier_id
		LEFT JOIN erp.warehouses w ON w.tenant_id = o.tenant_id AND w.id = o.warehouse_id
		WHERE ` + where
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	dataQuery := orderSelectSQL + ` WHERE ` + where +
		fmt.Sprintf(` ORDER BY o.order_date DESC, o.created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.db.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	orders := make([]*domain.Order, 0)
	for rows.Next() {
		order, scanErr := scanOrderHeader(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		lines, lineErr := loadOrderLines(ctx, r.db, tenantID, order.ID)
		if lineErr != nil {
			return nil, 0, lineErr
		}
		order.Lines = lines
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate orders: %w", err)
	}
	return orders, total, nil
}

func (r *Repository) UpdateStatus(
	ctx context.Context,
	filter scope.Filter,
	tenantID, id string,
	from, to domain.Status,
	userID string,
) (bool, error) {
	args := []any{tenantID, id, string(to), userID, string(from)}
	where := "o.tenant_id = $1 AND o.id = $2 AND o.status = $5"
	appendScope(&where, &args, "o", filter)

	tag, err := r.db.Exec(ctx, `
		UPDATE erp.purchase_orders AS o
		SET status = $3, updated_by = $4, updated_at = now()
		WHERE `+where, args...)
	if err != nil {
		return false, fmt.Errorf("update order status: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *Repository) Receive(ctx context.Context, filter scope.Filter, receipt *domain.Receipt) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin receive: %w", err)
	}
	defer tx.Rollback(ctx)

	args := []any{receipt.TenantID, receipt.OrderID}
	where := "o.tenant_id = $1 AND o.id = $2"
	appendScope(&where, &args, "o", filter)
	row := tx.QueryRow(ctx, orderSelectSQL+` WHERE `+where+` FOR UPDATE OF o`, args...)
	order, err := scanOrderHeader(row)
	if err != nil {
		return err
	}
	if !order.Status.CanReceive() {
		return domain.ErrInvalidStatus
	}
	if receipt.WarehouseID != order.WarehouseID || receipt.OrgID != order.OrgID {
		return domain.ErrInvalidInput
	}

	lines, err := loadOrderLinesForUpdate(ctx, tx, receipt.TenantID, receipt.OrderID)
	if err != nil {
		return err
	}
	lineByID := make(map[string]*domain.OrderLine, len(lines))
	for _, line := range lines {
		lineByID[line.ID] = line
	}

	newReceived := make(map[string]decimal.Decimal, len(lines))
	for _, line := range lines {
		newReceived[line.ID] = line.ReceivedQty
	}
	seenLines := make(map[string]struct{}, len(receipt.Lines))
	for _, line := range receipt.Lines {
		if !line.Quantity.IsPositive() {
			return domain.ErrInvalidInput
		}
		if _, exists := seenLines[line.OrderLineID]; exists {
			return domain.ErrInvalidInput
		}
		seenLines[line.OrderLineID] = struct{}{}
		orderLine, ok := lineByID[line.OrderLineID]
		if !ok {
			return domain.ErrInvalidInput
		}
		if line.Quantity.GreaterThan(orderLine.Remaining()) {
			return domain.ErrOverReceive
		}
		newReceived[orderLine.ID] = newReceived[orderLine.ID].Add(line.Quantity)
	}

	// Process inventory rows in a stable order to reduce deadlock risk for
	// multi-material concurrent receipts.
	sort.Slice(receipt.Lines, func(i, j int) bool {
		if receipt.Lines[i].MaterialID == receipt.Lines[j].MaterialID {
			return receipt.Lines[i].OrderLineID < receipt.Lines[j].OrderLineID
		}
		return receipt.Lines[i].MaterialID < receipt.Lines[j].MaterialID
	})

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.purchase_receipts (
			id, tenant_id, org_id, code, order_id, warehouse_id, receipt_date, status, remark, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		receipt.ID, receipt.TenantID, receipt.OrgID, receipt.Code, receipt.OrderID,
		receipt.WarehouseID, receipt.ReceiptDate, receipt.Status, receipt.Remark, receipt.CreatedBy,
	); err != nil {
		return fmt.Errorf("insert receipt: %w", mapDBError(err))
	}

	invoiceTotal := decimal.Zero
	for _, line := range receipt.Lines {
		invoiceTotal = invoiceTotal.Add(line.Amount)
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.purchase_receipt_lines (
				id, tenant_id, receipt_id, order_line_id, material_id, quantity, unit_price, amount
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			line.ID, line.TenantID, receipt.ID, line.OrderLineID, line.MaterialID,
			line.Quantity.String(), line.UnitPrice.String(), line.Amount.String(),
		); err != nil {
			return fmt.Errorf("insert receipt line: %w", mapDBError(err))
		}
		tag, updateErr := tx.Exec(ctx, `
			UPDATE erp.purchase_order_lines
			SET received_qty = received_qty + $3
			WHERE tenant_id = $1 AND id = $2
			  AND received_qty + $3 <= quantity`,
			receipt.TenantID, line.OrderLineID, line.Quantity.String(),
		)
		if updateErr != nil {
			return fmt.Errorf("update received quantity: %w", updateErr)
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrOverReceive
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.inventory_balances (
				tenant_id, org_id, warehouse_id, material_id, on_hand, reserved, available, avg_cost, updated_at
			) VALUES ($1,$2,$3,$4,$5,0,$5,$6,now())
			ON CONFLICT (tenant_id, warehouse_id, material_id) DO UPDATE
			SET avg_cost = CASE
			        WHEN erp.inventory_balances.on_hand + EXCLUDED.on_hand = 0 THEN 0
			        ELSE (erp.inventory_balances.on_hand * erp.inventory_balances.avg_cost
			              + EXCLUDED.on_hand * EXCLUDED.avg_cost)
			             / (erp.inventory_balances.on_hand + EXCLUDED.on_hand)
			    END,
			    on_hand = erp.inventory_balances.on_hand + EXCLUDED.on_hand,
			    available = erp.inventory_balances.available + EXCLUDED.available,
			    updated_at = now()`,
			receipt.TenantID, receipt.OrgID, receipt.WarehouseID, line.MaterialID,
			line.Quantity.String(), line.UnitPrice.String(),
		); err != nil {
			return fmt.Errorf("upsert inventory balance: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.inventory_transactions (
				id, tenant_id, org_id, warehouse_id, material_id, direction, biz_type,
				biz_id, biz_code, quantity, unit_cost, amount, occurred_at, created_by
			) VALUES ($1,$2,$3,$4,$5,'in','purchase_receipt',$6,$7,$8,$9,$10,now(),$11)`,
			uuid.NewString(), receipt.TenantID, receipt.OrgID, receipt.WarehouseID, line.MaterialID,
			receipt.ID, receipt.Code, line.Quantity.String(), line.UnitPrice.String(), line.Amount.String(), receipt.CreatedBy,
		); err != nil {
			return fmt.Errorf("insert inventory transaction: %w", err)
		}
	}

	invoiceTotal = invoiceTotal.Round(4)
	invoiceStatus := "open"
	if !invoiceTotal.IsPositive() {
		invoiceStatus = "paid"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.finance_invoices (
			id, tenant_id, org_id, code, invoice_type, source_type, source_id, source_code,
			partner_id, invoice_date, due_date, currency, status, total_amount, paid_amount,
			remark, created_by, updated_by
		) VALUES ($1,$2,$3,$4,'payable','purchase_receipt',$5,$6,$7,$8,$9,$10,$13,$11,0,'',$12,$12)`,
		receipt.PayableID, receipt.TenantID, receipt.OrgID, receipt.PayableCode,
		receipt.ID, receipt.Code, order.SupplierID, receipt.ReceiptDate,
		receipt.PayableDueDate, order.Currency, invoiceTotal.String(), receipt.CreatedBy,
		invoiceStatus,
	); err != nil {
		return fmt.Errorf("insert payable invoice: %w", mapDBError(err))
	}

	allReceived := true
	for _, line := range lines {
		received := newReceived[line.ID]
		if received.LessThan(line.Quantity) {
			allReceived = false
			break
		}
	}
	newStatus := domain.StatusPartiallyReceived
	if allReceived {
		newStatus = domain.StatusReceived
	}
	if _, err := tx.Exec(ctx, `
		UPDATE erp.purchase_orders
		SET status = $3, updated_by = $4, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		receipt.TenantID, receipt.OrderID, string(newStatus), receipt.CreatedBy,
	); err != nil {
		return fmt.Errorf("update order after receive: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit receive: %w", err)
	}
	return nil
}

func (r *Repository) ListReceipts(
	ctx context.Context,
	filter scope.Filter,
	tenantID, orderID string,
	offset, limit int,
) ([]*domain.Receipt, int, error) {
	args := []any{tenantID}
	where := "r.tenant_id = $1"
	if orderID != "" {
		args = append(args, orderID)
		where += fmt.Sprintf(" AND r.order_id = $%d", len(args))
	}
	appendScope(&where, &args, "r", filter)

	countQuery := `SELECT count(*) FROM erp.purchase_receipts r WHERE ` + where
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count receipts: %w", err)
	}

	query := receiptSelectSQL + ` WHERE ` + where +
		fmt.Sprintf(` ORDER BY r.receipt_date DESC, r.created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list receipts: %w", err)
	}
	defer rows.Close()

	receipts := make([]*domain.Receipt, 0)
	for rows.Next() {
		receipt, scanErr := scanReceipt(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		lines, lineErr := loadReceiptLines(ctx, r.db, tenantID, receipt.ID)
		if lineErr != nil {
			return nil, 0, lineErr
		}
		receipt.Lines = lines
		receipts = append(receipts, receipt)
	}
	return receipts, total, rows.Err()
}

const orderSelectSQL = `
	SELECT o.id::text, o.tenant_id::text, o.org_id::text, o.code,
	       o.supplier_id::text, COALESCE(p.name, ''), o.warehouse_id::text, COALESCE(w.name, ''),
	       o.order_date, o.currency, o.status, o.total_amount::text, o.remark,
	       o.created_by::text, o.updated_by::text, o.created_at, o.updated_at
	FROM erp.purchase_orders o
	LEFT JOIN erp.partners p ON p.tenant_id = o.tenant_id AND p.id = o.supplier_id
	LEFT JOIN erp.warehouses w ON w.tenant_id = o.tenant_id AND w.id = o.warehouse_id`

const receiptSelectSQL = `
	SELECT r.id::text, r.tenant_id::text, r.org_id::text, r.code,
	       r.order_id::text, o.code, COALESCE(p.name, ''), r.warehouse_id::text, COALESCE(w.name, ''),
	       r.receipt_date, r.status, r.remark, r.created_by::text, r.created_at,
	       COALESCE(i.code, ''), i.due_date
	FROM erp.purchase_receipts r
	JOIN erp.purchase_orders o ON o.tenant_id = r.tenant_id AND o.id = r.order_id
	LEFT JOIN erp.partners p ON p.tenant_id = o.tenant_id AND p.id = o.supplier_id
	LEFT JOIN erp.warehouses w ON w.tenant_id = r.tenant_id AND w.id = r.warehouse_id
	LEFT JOIN erp.finance_invoices i ON i.tenant_id = r.tenant_id
	  AND i.source_type = 'purchase_receipt' AND i.source_id = r.id AND i.invoice_type = 'payable'`

func insertLines(ctx context.Context, tx pgx.Tx, order *domain.Order) error {
	for _, line := range order.Lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.purchase_order_lines (
				id, tenant_id, order_id, line_no, material_id, quantity, unit_price, amount, received_qty
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			line.ID, line.TenantID, order.ID, line.LineNo, line.MaterialID,
			line.Quantity.String(), line.UnitPrice.String(), line.Amount.String(), line.ReceivedQty.String(),
		); err != nil {
			return fmt.Errorf("insert order line: %w", mapDBError(err))
		}
	}
	return nil
}

func loadOrderLines(ctx context.Context, q querier, tenantID, orderID string) ([]*domain.OrderLine, error) {
	rows, err := q.Query(ctx, `
		SELECT l.id::text, l.tenant_id::text, l.order_id::text, l.line_no,
		       l.material_id::text, COALESCE(m.name, ''), l.quantity::text,
		       l.unit_price::text, l.amount::text, l.received_qty::text
		FROM erp.purchase_order_lines l
		LEFT JOIN erp.materials m ON m.tenant_id = l.tenant_id AND m.id = l.material_id
		WHERE l.tenant_id = $1 AND l.order_id = $2
		ORDER BY l.line_no`, tenantID, orderID)
	if err != nil {
		return nil, fmt.Errorf("list order lines: %w", err)
	}
	defer rows.Close()

	lines := make([]*domain.OrderLine, 0)
	for rows.Next() {
		line, scanErr := scanOrderLine(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func loadOrderLinesForUpdate(ctx context.Context, tx pgx.Tx, tenantID, orderID string) ([]*domain.OrderLine, error) {
	rows, err := tx.Query(ctx, `
		SELECT l.id::text, l.tenant_id::text, l.order_id::text, l.line_no,
		       l.material_id::text, COALESCE(m.name, ''), l.quantity::text,
		       l.unit_price::text, l.amount::text, l.received_qty::text
		FROM erp.purchase_order_lines l
		LEFT JOIN erp.materials m ON m.tenant_id = l.tenant_id AND m.id = l.material_id
		WHERE l.tenant_id = $1 AND l.order_id = $2
		ORDER BY l.line_no
		FOR UPDATE OF l`, tenantID, orderID)
	if err != nil {
		return nil, fmt.Errorf("lock order lines: %w", err)
	}
	defer rows.Close()

	lines := make([]*domain.OrderLine, 0)
	for rows.Next() {
		line, scanErr := scanOrderLine(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func loadReceiptLines(ctx context.Context, q querier, tenantID, receiptID string) ([]*domain.ReceiptLine, error) {
	rows, err := q.Query(ctx, `
		SELECT rl.id::text, rl.tenant_id::text, rl.receipt_id::text, rl.order_line_id::text,
		       rl.material_id::text, COALESCE(m.name, ''), rl.quantity::text,
		       rl.unit_price::text, rl.amount::text
		FROM erp.purchase_receipt_lines rl
		LEFT JOIN erp.materials m ON m.tenant_id = rl.tenant_id AND m.id = rl.material_id
		WHERE rl.tenant_id = $1 AND rl.receipt_id = $2
		ORDER BY rl.id`, tenantID, receiptID)
	if err != nil {
		return nil, fmt.Errorf("list receipt lines: %w", err)
	}
	defer rows.Close()

	lines := make([]*domain.ReceiptLine, 0)
	for rows.Next() {
		line, scanErr := scanReceiptLine(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanOrderHeader(row rowScanner) (*domain.Order, error) {
	var order domain.Order
	var status string
	var totalAmount string

	err := row.Scan(
		&order.ID, &order.TenantID, &order.OrgID, &order.Code,
		&order.SupplierID, &order.SupplierName, &order.WarehouseID, &order.WarehouseName,
		&order.OrderDate, &order.Currency, &status, &totalAmount, &order.Remark,
		&order.CreatedBy, &order.UpdatedBy, &order.CreatedAt, &order.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan order: %w", err)
	}
	order.Status = domain.Status(status)
	order.TotalAmount = decimal.RequireFromString(totalAmount)
	return &order, nil
}

func scanOrderLine(row rowScanner) (*domain.OrderLine, error) {
	var line domain.OrderLine
	var quantity, unitPrice, amount, receivedQty string

	err := row.Scan(
		&line.ID, &line.TenantID, &line.OrderID, &line.LineNo,
		&line.MaterialID, &line.MaterialName, &quantity, &unitPrice, &amount, &receivedQty,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan order line: %w", err)
	}
	line.Quantity = decimal.RequireFromString(quantity)
	line.UnitPrice = decimal.RequireFromString(unitPrice)
	line.Amount = decimal.RequireFromString(amount)
	line.ReceivedQty = decimal.RequireFromString(receivedQty)
	return &line, nil
}

func scanReceipt(row rowScanner) (*domain.Receipt, error) {
	var receipt domain.Receipt
	var payableDueDate sql.NullTime
	err := row.Scan(
		&receipt.ID, &receipt.TenantID, &receipt.OrgID, &receipt.Code,
		&receipt.OrderID, &receipt.OrderCode, &receipt.SupplierName,
		&receipt.WarehouseID, &receipt.WarehouseName, &receipt.ReceiptDate,
		&receipt.Status, &receipt.Remark, &receipt.CreatedBy, &receipt.CreatedAt,
		&receipt.PayableCode, &payableDueDate,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan receipt: %w", err)
	}
	if payableDueDate.Valid {
		receipt.PayableDueDate = payableDueDate.Time
	}
	return &receipt, nil
}

func scanReceiptLine(row rowScanner) (*domain.ReceiptLine, error) {
	var line domain.ReceiptLine
	var quantity, unitPrice, amount string
	err := row.Scan(
		&line.ID, &line.TenantID, &line.ReceiptID, &line.OrderLineID,
		&line.MaterialID, &line.MaterialName, &quantity, &unitPrice, &amount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan receipt line: %w", err)
	}
	line.Quantity = decimal.RequireFromString(quantity)
	line.UnitPrice = decimal.RequireFromString(unitPrice)
	line.Amount = decimal.RequireFromString(amount)
	return &line, nil
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func appendScope(where *string, args *[]any, alias string, filter scope.Filter) {
	if filter.DataScope == "all" {
		return
	}
	if filter.DataScope == "" {
		filter.DataScope = "self"
	}

	switch filter.DataScope {
	case "self":
		if filter.UserID == "" {
			*where += " AND 1 = 0"
			return
		}
		*args = append(*args, filter.UserID)
		*where += fmt.Sprintf(" AND %s.created_by = $%d", alias, len(*args))
	case "dept":
		if filter.OrgPath == "" {
			*where += " AND 1 = 0"
			return
		}
		*args = append(*args, filter.OrgPath)
		*where += fmt.Sprintf(
			" AND EXISTS (SELECT 1 FROM erp.organizations so WHERE so.tenant_id = %s.tenant_id AND so.id = %s.org_id AND (so.path = $%d OR so.path LIKE $%d || '/%%'))",
			alias, alias, len(*args), len(*args),
		)
	case "company":
		if filter.CompanyPath == "" {
			*where += " AND 1 = 0"
			return
		}
		*args = append(*args, filter.CompanyPath)
		*where += fmt.Sprintf(
			" AND EXISTS (SELECT 1 FROM erp.organizations so WHERE so.tenant_id = %s.tenant_id AND so.id = %s.org_id AND (so.path = $%d OR so.path LIKE $%d || '/%%'))",
			alias, alias, len(*args), len(*args),
		)
	default:
		if filter.UserID == "" {
			*where += " AND 1 = 0"
			return
		}
		*args = append(*args, filter.UserID)
		*where += fmt.Sprintf(" AND %s.created_by = $%d", alias, len(*args))
	}
}

func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "23503":
			return domain.ErrConflict
		case "22P02", "23502", "22003", "23514":
			return domain.ErrInvalidInput
		}
	}
	return err
}
