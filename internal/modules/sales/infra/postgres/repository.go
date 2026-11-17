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
	glpost "github.com/mlf-erp/mlf-erp/internal/modules/gl/infra/postgres"
	"github.com/mlf-erp/mlf-erp/internal/modules/sales/domain"
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
		return fmt.Errorf("begin create sales order: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.sales_orders (
			id, tenant_id, org_id, code, customer_id, warehouse_id, order_date,
			currency, status, price_includes_tax, net_amount, tax_amount, total_amount,
			remark, created_by, updated_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		order.ID, order.TenantID, order.OrgID, order.Code, order.CustomerID, order.WarehouseID,
		order.OrderDate, order.Currency, string(order.Status), order.PriceIncludesTax,
		order.NetAmount.String(), order.TaxAmount.String(), order.TotalAmount.String(),
		order.Remark, order.CreatedBy, order.UpdatedBy,
	); err != nil {
		return fmt.Errorf("insert sales order: %w", mapDBError(err))
	}
	if err := insertLines(ctx, tx, order); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create sales order: %w", err)
	}
	return nil
}

func (r *Repository) UpdateDraft(ctx context.Context, filter scope.Filter, order *domain.Order) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update sales order: %w", err)
	}
	defer tx.Rollback(ctx)

	args := []any{
		order.TenantID, order.ID, order.CustomerID, order.WarehouseID, order.OrderDate,
		order.Currency, order.PriceIncludesTax, order.NetAmount.String(), order.TaxAmount.String(),
		order.TotalAmount.String(), order.Remark, order.UpdatedBy,
	}
	where := "o.tenant_id = $1 AND o.id = $2 AND o.status = 'draft'"
	appendScope(&where, &args, "o", filter)

	tag, err := tx.Exec(ctx, `
		UPDATE erp.sales_orders AS o
		SET customer_id = $3, warehouse_id = $4, order_date = $5,
		    currency = $6, price_includes_tax = $7, net_amount = $8, tax_amount = $9,
		    total_amount = $10, remark = $11, updated_by = $12, updated_at = now()
		WHERE `+where, args...)
	if err != nil {
		return fmt.Errorf("update sales order: %w", mapDBError(err))
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM erp.sales_order_lines WHERE tenant_id = $1 AND order_id = $2`, order.TenantID, order.ID); err != nil {
		return fmt.Errorf("delete sales order lines: %w", err)
	}
	if err := insertLines(ctx, tx, order); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit update sales order: %w", err)
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

	countQuery := `SELECT count(*) FROM erp.sales_orders o
		LEFT JOIN erp.partners p ON p.tenant_id = o.tenant_id AND p.id = o.customer_id
		LEFT JOIN erp.warehouses w ON w.tenant_id = o.tenant_id AND w.id = o.warehouse_id
		WHERE ` + where
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sales orders: %w", err)
	}

	dataQuery := orderSelectSQL + ` WHERE ` + where +
		fmt.Sprintf(` ORDER BY o.order_date DESC, o.created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.db.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list sales orders: %w", err)
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
		return nil, 0, fmt.Errorf("iterate sales orders: %w", err)
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
		UPDATE erp.sales_orders AS o
		SET status = $3, updated_by = $4, updated_at = now()
		WHERE `+where, args...)
	if err != nil {
		return false, fmt.Errorf("update sales order status: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *Repository) Ship(ctx context.Context, filter scope.Filter, shipment *domain.Shipment) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin ship: %w", err)
	}
	defer tx.Rollback(ctx)

	args := []any{shipment.TenantID, shipment.OrderID}
	where := "o.tenant_id = $1 AND o.id = $2"
	appendScope(&where, &args, "o", filter)
	row := tx.QueryRow(ctx, orderSelectSQL+` WHERE `+where+` FOR UPDATE OF o`, args...)
	order, err := scanOrderHeader(row)
	if err != nil {
		return err
	}
	if !order.Status.CanShip() {
		return domain.ErrInvalidStatus
	}
	if shipment.WarehouseID != order.WarehouseID || shipment.OrgID != order.OrgID ||
		shipment.CustomerID != order.CustomerID {
		return domain.ErrInvalidInput
	}

	lines, err := loadOrderLinesForUpdate(ctx, tx, shipment.TenantID, shipment.OrderID)
	if err != nil {
		return err
	}
	lineByID := make(map[string]*domain.OrderLine, len(lines))
	for _, line := range lines {
		lineByID[line.ID] = line
	}

	newShipped := make(map[string]decimal.Decimal, len(lines))
	for _, line := range lines {
		newShipped[line.ID] = line.ShippedQty
	}
	seenLines := make(map[string]struct{}, len(shipment.Lines))
	for _, line := range shipment.Lines {
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
			return domain.ErrOverShipment
		}
		newShipped[orderLine.ID] = newShipped[orderLine.ID].Add(line.Quantity)
	}

	// Process inventory rows in a stable order to reduce deadlock risk for
	// multi-material concurrent shipments.
	sort.Slice(shipment.Lines, func(i, j int) bool {
		if shipment.Lines[i].MaterialID == shipment.Lines[j].MaterialID {
			return shipment.Lines[i].OrderLineID < shipment.Lines[j].OrderLineID
		}
		return shipment.Lines[i].MaterialID < shipment.Lines[j].MaterialID
	})

	type balanceKey struct {
		warehouseID string
		materialID  string
	}
	balances := make(map[balanceKey]decimal.Decimal, len(shipment.Lines))
	for _, line := range shipment.Lines {
		key := balanceKey{warehouseID: shipment.WarehouseID, materialID: line.MaterialID}
		if _, loaded := balances[key]; loaded {
			continue
		}
		var onHandRaw, reservedRaw, availableRaw, avgCostRaw string
		err := tx.QueryRow(ctx, `
			SELECT on_hand::text, reserved::text, available::text, avg_cost::text
			FROM erp.inventory_balances
			WHERE tenant_id = $1 AND warehouse_id = $2 AND material_id = $3
			FOR UPDATE`,
			shipment.TenantID, shipment.WarehouseID, line.MaterialID,
		).Scan(&onHandRaw, &reservedRaw, &availableRaw, &avgCostRaw)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInsufficientStock
		}
		if err != nil {
			return fmt.Errorf("lock inventory balance: %w", err)
		}
		available := decimal.RequireFromString(availableRaw)
		if available.LessThan(line.Quantity) {
			return domain.ErrInsufficientStock
		}
		balances[key] = decimal.RequireFromString(avgCostRaw)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.sales_shipments (
			id, tenant_id, org_id, code, order_id, warehouse_id, shipment_date, status,
			price_includes_tax, net_amount, tax_amount, total_amount, remark, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		shipment.ID, shipment.TenantID, shipment.OrgID, shipment.Code, shipment.OrderID,
		shipment.WarehouseID, shipment.ShipmentDate, shipment.Status,
		shipment.PriceIncludesTax, shipment.NetAmount.String(), shipment.TaxAmount.String(),
		shipment.GrossAmount.String(), shipment.Remark, shipment.CreatedBy,
	); err != nil {
		return fmt.Errorf("insert shipment: %w", mapDBError(err))
	}

	invoiceTotal := decimal.Zero
	netTotal := decimal.Zero
	taxTotal := decimal.Zero
	costTotal := decimal.Zero
	for _, line := range shipment.Lines {
		key := balanceKey{warehouseID: shipment.WarehouseID, materialID: line.MaterialID}
		unitCost := balances[key]
		line.UnitCost = unitCost
		line.CostAmount = line.Quantity.Mul(unitCost).Round(4)
		netTotal = netTotal.Add(line.Amount)
		taxTotal = taxTotal.Add(line.TaxAmount)
		invoiceTotal = invoiceTotal.Add(line.GrossAmount)
		costTotal = costTotal.Add(line.CostAmount)

		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.sales_shipment_lines (
				id, tenant_id, shipment_id, order_line_id, material_id,
				quantity, unit_price, tax_rate_id, tax_rate, amount, tax_amount, gross_amount,
				unit_cost, cost_amount
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			line.ID, line.TenantID, shipment.ID, line.OrderLineID, line.MaterialID,
			line.Quantity.String(), line.UnitPrice.String(), nullableUUID(line.TaxRateID),
			line.TaxRate.String(), line.Amount.String(), line.TaxAmount.String(),
			line.GrossAmount.String(), line.UnitCost.String(), line.CostAmount.String(),
		); err != nil {
			return fmt.Errorf("insert shipment line: %w", mapDBError(err))
		}

		tag, updateErr := tx.Exec(ctx, `
			UPDATE erp.sales_order_lines
			SET shipped_qty = shipped_qty + $3
			WHERE tenant_id = $1 AND id = $2
			  AND shipped_qty + $3 <= quantity`,
			shipment.TenantID, line.OrderLineID, line.Quantity.String(),
		)
		if updateErr != nil {
			return fmt.Errorf("update shipped quantity: %w", updateErr)
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrOverShipment
		}

		tag, updateErr = tx.Exec(ctx, `
			UPDATE erp.inventory_balances
			SET on_hand = on_hand - $4,
			    available = available - $4,
			    updated_at = now()
			WHERE tenant_id = $1 AND warehouse_id = $2 AND material_id = $3
			  AND on_hand >= $4 AND available >= $4`,
			shipment.TenantID, shipment.WarehouseID, line.MaterialID, line.Quantity.String(),
		)
		if updateErr != nil {
			return fmt.Errorf("deduct inventory balance: %w", updateErr)
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrInsufficientStock
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.inventory_transactions (
				id, tenant_id, org_id, warehouse_id, material_id, direction, biz_type,
				biz_id, biz_code, quantity, unit_cost, amount, occurred_at, created_by
			) VALUES ($1,$2,$3,$4,$5,'out','sales_shipment',$6,$7,$8,$9,$10,now(),$11)`,
			uuid.NewString(), shipment.TenantID, shipment.OrgID, shipment.WarehouseID, line.MaterialID,
			shipment.ID, shipment.Code, line.Quantity.String(), line.UnitCost.String(),
			line.CostAmount.String(), shipment.CreatedBy,
		); err != nil {
			return fmt.Errorf("insert inventory transaction: %w", err)
		}
	}

	invoiceTotal = invoiceTotal.Round(4)
	netTotal = netTotal.Round(4)
	taxTotal = taxTotal.Round(4)
	invoiceStatus := "open"
	if !invoiceTotal.IsPositive() {
		invoiceStatus = "paid"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.finance_invoices (
			id, tenant_id, org_id, code, invoice_type, source_type, source_id, source_code,
			partner_id, invoice_date, due_date, currency, status, net_amount, tax_amount,
			total_amount, paid_amount, vat_invoice_status, remark, created_by, updated_by
		) VALUES ($1,$2,$3,$4,'receivable','sales_shipment',$5,$6,$7,$8,$9,$10,$13,$14,$15,$11,0,'pending','',$12,$12)`,
		shipment.InvoiceID, shipment.TenantID, shipment.OrgID, shipment.InvoiceCode,
		shipment.ID, shipment.Code, order.CustomerID, shipment.ShipmentDate,
		shipment.InvoiceDueDate, order.Currency, invoiceTotal.String(),
		shipment.CreatedBy, invoiceStatus, netTotal.String(), taxTotal.String(),
	); err != nil {
		return fmt.Errorf("insert receivable invoice: %w", mapDBError(err))
	}

	if err := glpost.PostSalesShipment(ctx, tx, glpost.SalesShipmentPosting{
		TenantID:    shipment.TenantID,
		OrgID:       shipment.OrgID,
		VoucherCode: shipment.VoucherCode,
		SourceID:    shipment.ID,
		SourceCode:  shipment.Code,
		CustomerID:  order.CustomerID,
		CreatedBy:   shipment.CreatedBy,
		VoucherDate: shipment.ShipmentDate,
		NetAmount:   netTotal,
		TaxAmount:   taxTotal,
		GrossAmount: invoiceTotal,
		CostAmount:  costTotal,
	}); err != nil {
		return fmt.Errorf("post sales shipment voucher: %w", err)
	}

	allShipped := true
	for _, line := range lines {
		shipped := newShipped[line.ID]
		if shipped.LessThan(line.Quantity) {
			allShipped = false
			break
		}
	}
	newStatus := domain.StatusPartiallyShipped
	if allShipped {
		newStatus = domain.StatusShipped
	}
	if _, err := tx.Exec(ctx, `
		UPDATE erp.sales_orders
		SET status = $3, updated_by = $4, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		shipment.TenantID, shipment.OrderID, string(newStatus), shipment.CreatedBy,
	); err != nil {
		return fmt.Errorf("update sales order after shipment: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit ship: %w", err)
	}
	return nil
}

func (r *Repository) ListShipments(
	ctx context.Context,
	filter scope.Filter,
	tenantID, orderID string,
	offset, limit int,
) ([]*domain.Shipment, int, error) {
	args := []any{tenantID}
	where := "s.tenant_id = $1"
	if orderID != "" {
		args = append(args, orderID)
		where += fmt.Sprintf(" AND s.order_id = $%d", len(args))
	}
	appendScope(&where, &args, "s", filter)

	countQuery := `SELECT count(*) FROM erp.sales_shipments s WHERE ` + where
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count shipments: %w", err)
	}

	query := shipmentSelectSQL + ` WHERE ` + where +
		fmt.Sprintf(` ORDER BY s.shipment_date DESC, s.created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list shipments: %w", err)
	}
	defer rows.Close()

	shipments := make([]*domain.Shipment, 0)
	for rows.Next() {
		shipment, scanErr := scanShipment(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		lines, lineErr := loadShipmentLines(ctx, r.db, tenantID, shipment.ID)
		if lineErr != nil {
			return nil, 0, lineErr
		}
		shipment.Lines = lines
		shipments = append(shipments, shipment)
	}
	return shipments, total, rows.Err()
}

const orderSelectSQL = `
	SELECT o.id::text, o.tenant_id::text, o.org_id::text, o.code,
	       o.customer_id::text, COALESCE(p.name, ''), o.warehouse_id::text, COALESCE(w.name, ''),
	       o.order_date, o.currency, o.status, o.price_includes_tax,
	       o.net_amount::text, o.tax_amount::text, o.total_amount::text, o.remark,
	       o.created_by::text, o.updated_by::text, o.created_at, o.updated_at
	FROM erp.sales_orders o
	LEFT JOIN erp.partners p ON p.tenant_id = o.tenant_id AND p.id = o.customer_id
	LEFT JOIN erp.warehouses w ON w.tenant_id = o.tenant_id AND w.id = o.warehouse_id`

const shipmentSelectSQL = `
	SELECT s.id::text, s.tenant_id::text, s.org_id::text, s.code,
	       s.order_id::text, o.code, o.customer_id::text, COALESCE(p.name, ''),
	       s.warehouse_id::text, COALESCE(w.name, ''), s.shipment_date, s.status,
	       s.price_includes_tax, s.net_amount::text, s.tax_amount::text, s.total_amount::text,
	       s.remark, s.created_by::text, s.created_at,
	       COALESCE(i.id::text, ''), COALESCE(i.code, ''), i.due_date
	FROM erp.sales_shipments s
	JOIN erp.sales_orders o ON o.tenant_id = s.tenant_id AND o.id = s.order_id
	LEFT JOIN erp.partners p ON p.tenant_id = o.tenant_id AND p.id = o.customer_id
	LEFT JOIN erp.warehouses w ON w.tenant_id = s.tenant_id AND w.id = s.warehouse_id
	LEFT JOIN erp.finance_invoices i ON i.tenant_id = s.tenant_id
	  AND i.source_type = 'sales_shipment' AND i.source_id = s.id AND i.invoice_type = 'receivable'`

func insertLines(ctx context.Context, tx pgx.Tx, order *domain.Order) error {
	for _, line := range order.Lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.sales_order_lines (
				id, tenant_id, order_id, line_no, material_id, quantity, unit_price,
				tax_rate_id, tax_rate, amount, tax_amount, gross_amount, shipped_qty
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			line.ID, line.TenantID, order.ID, line.LineNo, line.MaterialID,
			line.Quantity.String(), line.UnitPrice.String(), nullableUUID(line.TaxRateID),
			line.TaxRate.String(), line.Amount.String(), line.TaxAmount.String(),
			line.GrossAmount.String(), line.ShippedQty.String(),
		); err != nil {
			return fmt.Errorf("insert sales order line: %w", mapDBError(err))
		}
	}
	return nil
}

func loadOrderLines(ctx context.Context, q querier, tenantID, orderID string) ([]*domain.OrderLine, error) {
	rows, err := q.Query(ctx, `
		SELECT l.id::text, l.tenant_id::text, l.order_id::text, l.line_no,
		       l.material_id::text, COALESCE(m.name, ''), l.quantity::text,
		       l.unit_price::text, COALESCE(l.tax_rate_id::text, ''), l.tax_rate::text,
		       l.amount::text, l.tax_amount::text, l.gross_amount::text, l.shipped_qty::text
		FROM erp.sales_order_lines l
		LEFT JOIN erp.materials m ON m.tenant_id = l.tenant_id AND m.id = l.material_id
		WHERE l.tenant_id = $1 AND l.order_id = $2
		ORDER BY l.line_no`, tenantID, orderID)
	if err != nil {
		return nil, fmt.Errorf("list sales order lines: %w", err)
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
		       l.unit_price::text, COALESCE(l.tax_rate_id::text, ''), l.tax_rate::text,
		       l.amount::text, l.tax_amount::text, l.gross_amount::text, l.shipped_qty::text
		FROM erp.sales_order_lines l
		LEFT JOIN erp.materials m ON m.tenant_id = l.tenant_id AND m.id = l.material_id
		WHERE l.tenant_id = $1 AND l.order_id = $2
		ORDER BY l.line_no
		FOR UPDATE OF l`, tenantID, orderID)
	if err != nil {
		return nil, fmt.Errorf("lock sales order lines: %w", err)
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

func loadShipmentLines(ctx context.Context, q querier, tenantID, shipmentID string) ([]*domain.ShipmentLine, error) {
	rows, err := q.Query(ctx, `
		SELECT sl.id::text, sl.tenant_id::text, sl.shipment_id::text, sl.order_line_id::text,
		       sl.material_id::text, COALESCE(m.name, ''), sl.quantity::text,
		       sl.unit_price::text, COALESCE(sl.tax_rate_id::text, ''), sl.tax_rate::text,
		       sl.amount::text, sl.tax_amount::text, sl.gross_amount::text,
		       sl.unit_cost::text, sl.cost_amount::text
		FROM erp.sales_shipment_lines sl
		LEFT JOIN erp.materials m ON m.tenant_id = sl.tenant_id AND m.id = sl.material_id
		WHERE sl.tenant_id = $1 AND sl.shipment_id = $2
		ORDER BY sl.id`, tenantID, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("list shipment lines: %w", err)
	}
	defer rows.Close()

	lines := make([]*domain.ShipmentLine, 0)
	for rows.Next() {
		line, scanErr := scanShipmentLine(rows)
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
	var netAmount, taxAmount, totalAmount string

	err := row.Scan(
		&order.ID, &order.TenantID, &order.OrgID, &order.Code,
		&order.CustomerID, &order.CustomerName, &order.WarehouseID, &order.WarehouseName,
		&order.OrderDate, &order.Currency, &status, &order.PriceIncludesTax,
		&netAmount, &taxAmount, &totalAmount, &order.Remark,
		&order.CreatedBy, &order.UpdatedBy, &order.CreatedAt, &order.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan sales order: %w", err)
	}
	order.Status = domain.Status(status)
	order.NetAmount = decimal.RequireFromString(netAmount)
	order.TaxAmount = decimal.RequireFromString(taxAmount)
	order.TotalAmount = decimal.RequireFromString(totalAmount)
	return &order, nil
}

func scanOrderLine(row rowScanner) (*domain.OrderLine, error) {
	var line domain.OrderLine
	var quantity, unitPrice, taxRate, amount, taxAmount, grossAmount, shippedQty string

	err := row.Scan(
		&line.ID, &line.TenantID, &line.OrderID, &line.LineNo,
		&line.MaterialID, &line.MaterialName, &quantity, &unitPrice,
		&line.TaxRateID, &taxRate, &amount, &taxAmount, &grossAmount, &shippedQty,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan sales order line: %w", err)
	}
	line.Quantity = decimal.RequireFromString(quantity)
	line.UnitPrice = decimal.RequireFromString(unitPrice)
	line.TaxRate = decimal.RequireFromString(taxRate)
	line.Amount = decimal.RequireFromString(amount)
	line.TaxAmount = decimal.RequireFromString(taxAmount)
	line.GrossAmount = decimal.RequireFromString(grossAmount)
	line.ShippedQty = decimal.RequireFromString(shippedQty)
	return &line, nil
}

func scanShipment(row rowScanner) (*domain.Shipment, error) {
	var shipment domain.Shipment
	var invoiceDueDate sql.NullTime
	var netAmount, taxAmount, totalAmount string
	err := row.Scan(
		&shipment.ID, &shipment.TenantID, &shipment.OrgID, &shipment.Code,
		&shipment.OrderID, &shipment.OrderCode, &shipment.CustomerID, &shipment.CustomerName,
		&shipment.WarehouseID, &shipment.WarehouseName, &shipment.ShipmentDate,
		&shipment.Status, &shipment.PriceIncludesTax, &netAmount, &taxAmount, &totalAmount,
		&shipment.Remark, &shipment.CreatedBy, &shipment.CreatedAt,
		&shipment.InvoiceID, &shipment.InvoiceCode, &invoiceDueDate,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan shipment: %w", err)
	}
	shipment.NetAmount = decimal.RequireFromString(netAmount)
	shipment.TaxAmount = decimal.RequireFromString(taxAmount)
	shipment.GrossAmount = decimal.RequireFromString(totalAmount)
	if invoiceDueDate.Valid {
		shipment.InvoiceDueDate = invoiceDueDate.Time
	}
	return &shipment, nil
}

func scanShipmentLine(row rowScanner) (*domain.ShipmentLine, error) {
	var line domain.ShipmentLine
	var quantity, unitPrice, taxRate, amount, taxAmount, grossAmount, unitCost, costAmount string
	err := row.Scan(
		&line.ID, &line.TenantID, &line.ShipmentID, &line.OrderLineID,
		&line.MaterialID, &line.MaterialName, &quantity, &unitPrice,
		&line.TaxRateID, &taxRate, &amount, &taxAmount, &grossAmount,
		&unitCost, &costAmount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan shipment line: %w", err)
	}
	line.Quantity = decimal.RequireFromString(quantity)
	line.UnitPrice = decimal.RequireFromString(unitPrice)
	line.TaxRate = decimal.RequireFromString(taxRate)
	line.Amount = decimal.RequireFromString(amount)
	line.TaxAmount = decimal.RequireFromString(taxAmount)
	line.GrossAmount = decimal.RequireFromString(grossAmount)
	line.UnitCost = decimal.RequireFromString(unitCost)
	line.CostAmount = decimal.RequireFromString(costAmount)
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

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}
