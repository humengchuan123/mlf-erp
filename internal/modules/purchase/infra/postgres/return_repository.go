package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	glpost "github.com/mlf-erp/mlf-erp/internal/modules/gl/infra/postgres"
	"github.com/mlf-erp/mlf-erp/internal/modules/purchase/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type ReturnRepository struct {
	db *pgxpool.Pool
}

func NewReturnRepository(db *pgxpool.Pool) *ReturnRepository {
	return &ReturnRepository{db: db}
}

func (r *ReturnRepository) Create(
	ctx context.Context,
	filter scope.Filter,
	tenantID, userID string,
	params domain.ReturnCreateParams,
) (*domain.PurchaseReturn, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create purchase return: %w", err)
	}
	defer tx.Rollback(ctx)

	args := []any{tenantID, params.ReceiptID}
	where := "r.tenant_id = $1 AND r.id = $2"
	appendScope(&where, &args, "r", filter)

	var receiptID, orgID, receiptCode, orderID, orderCode, supplierID, warehouseID, status, currency string
	var exchangeRateRaw string
	var receiptDate time.Time
	var priceIncludesTax bool
	err = tx.QueryRow(ctx, `
		SELECT r.id::text, r.org_id::text, r.code, r.order_id::text, o.code,
		       o.supplier_id::text, r.warehouse_id::text, r.receipt_date, r.status,
		       r.exchange_rate::text, r.price_includes_tax, o.currency
		FROM erp.purchase_receipts r
		JOIN erp.purchase_orders o ON o.tenant_id = r.tenant_id AND o.id = r.order_id
		WHERE `+where+`
		FOR UPDATE OF r, o`, args...).Scan(
		&receiptID, &orgID, &receiptCode, &orderID, &orderCode,
		&supplierID, &warehouseID, &receiptDate, &status,
		&exchangeRateRaw, &priceIncludesTax, &currency,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock purchase receipt: %w", err)
	}
	if status != "posted" {
		return nil, domain.ErrInvalidStatus
	}
	if params.ReturnDate.IsZero() ||
		params.ReturnDate.Format("2006-01-02") < receiptDate.Format("2006-01-02") {
		return nil, domain.ErrInvalidInput
	}

	lineRows, err := tx.Query(ctx, `
		SELECT rl.id::text, rl.tenant_id::text, rl.receipt_id::text, rl.order_line_id::text,
		       rl.material_id::text, COALESCE(m.name, ''), rl.quantity::text,
		       rl.unit_price::text, COALESCE(rl.tax_rate_id::text, ''), rl.tax_rate::text,
		       rl.amount::text, rl.tax_amount::text, rl.gross_amount::text,
		       rl.base_amount::text, rl.base_tax_amount::text, rl.base_gross_amount::text,
		       rl.returned_qty::text
		FROM erp.purchase_receipt_lines rl
		LEFT JOIN erp.materials m ON m.tenant_id = rl.tenant_id AND m.id = rl.material_id
		WHERE rl.tenant_id = $1 AND rl.receipt_id = $2
		ORDER BY rl.id
		FOR UPDATE OF rl`, tenantID, params.ReceiptID)
	if err != nil {
		return nil, fmt.Errorf("lock purchase receipt lines: %w", err)
	}
	defer lineRows.Close()

	receiptLines := make([]*domain.ReceiptLine, 0)
	for lineRows.Next() {
		line, scanErr := scanReceiptLine(lineRows)
		if scanErr != nil {
			return nil, scanErr
		}
		receiptLines = append(receiptLines, line)
	}
	if err := lineRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate purchase receipt lines: %w", err)
	}
	lineRows.Close()

	receipt := &domain.Receipt{
		ID:               receiptID,
		TenantID:         tenantID,
		OrgID:            orgID,
		Code:             receiptCode,
		OrderID:          orderID,
		OrderCode:        orderCode,
		WarehouseID:      warehouseID,
		ReceiptDate:      receiptDate,
		Status:           status,
		PriceIncludesTax: priceIncludesTax,
		ExchangeRate:     decimal.RequireFromString(exchangeRateRaw),
		Lines:            receiptLines,
	}

	returnLines, err := domain.BuildReturnLines(receipt, params.Lines)
	if err != nil {
		return nil, err
	}

	ret := &domain.PurchaseReturn{
		ID:               uuid.NewString(),
		TenantID:         tenantID,
		OrgID:            orgID,
		Code:             params.ReturnCode,
		ReceiptID:        receiptID,
		ReceiptCode:      receiptCode,
		OrderID:          orderID,
		OrderCode:        orderCode,
		SupplierID:       supplierID,
		WarehouseID:      warehouseID,
		ReturnDate:       params.ReturnDate,
		Status:           "posted",
		Currency:         currency,
		ExchangeRate:     receipt.ExchangeRate,
		PriceIncludesTax: priceIncludesTax,
		VoucherCode:      params.VoucherCode,
		Reason:           params.Reason,
		Remark:           params.Remark,
		CreatedBy:        userID,
		Lines:            returnLines,
	}
	for _, line := range ret.Lines {
		line.ID = uuid.NewString()
		line.ReturnID = ret.ID
	}
	ret.RecalculateTotals()
	if ret.BaseTotalAmount.IsZero() && ret.BaseNetAmount.IsZero() && ret.BaseTaxAmount.IsZero() {
		ret.VoucherCode = ""
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.purchase_returns (
			id, tenant_id, org_id, code, receipt_id, receipt_code, order_id, order_code,
			supplier_id, warehouse_id, return_date, status, currency, exchange_rate,
			price_includes_tax, net_amount, tax_amount, total_amount,
			base_net_amount, base_tax_amount, base_total_amount, cost_amount,
			credit_note_id, voucher_code, reason, remark, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,
		          $19,$20,$21,$22,NULL,$23,$24,$25,$26)`,
		ret.ID, ret.TenantID, ret.OrgID, ret.Code, ret.ReceiptID, ret.ReceiptCode,
		ret.OrderID, ret.OrderCode, ret.SupplierID, ret.WarehouseID, ret.ReturnDate,
		ret.Status, ret.Currency, ret.ExchangeRate.String(), ret.PriceIncludesTax,
		ret.NetAmount.String(), ret.TaxAmount.String(), ret.TotalAmount.String(),
		ret.BaseNetAmount.String(), ret.BaseTaxAmount.String(), ret.BaseTotalAmount.String(),
		ret.CostAmount.String(), ret.VoucherCode, ret.Reason, ret.Remark, ret.CreatedBy,
	); err != nil {
		return nil, fmt.Errorf("insert purchase return: %w", mapDBError(err))
	}

	for _, line := range ret.Lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.purchase_return_lines (
				id, tenant_id, return_id, line_no, receipt_line_id, order_line_id,
				material_id, quantity, unit_price, tax_rate_id, tax_rate,
				amount, tax_amount, gross_amount, base_amount, base_tax_amount,
				base_gross_amount, unit_cost, cost_amount
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
			line.ID, tenantID, ret.ID, line.LineNo, line.ReceiptLineID, line.OrderLineID,
			line.MaterialID, line.Quantity.String(), line.UnitPrice.String(),
			nullableUUID(line.TaxRateID), line.TaxRate.String(), line.Amount.String(),
			line.TaxAmount.String(), line.GrossAmount.String(), line.BaseAmount.String(),
			line.BaseTaxAmount.String(), line.BaseGrossAmount.String(), line.UnitCost.String(),
			line.CostAmount.String(),
		); err != nil {
			return nil, fmt.Errorf("insert purchase return line: %w", mapDBError(err))
		}
	}

	for _, line := range ret.Lines {
		tag, updateErr := tx.Exec(ctx, `
			UPDATE erp.purchase_receipt_lines
			SET returned_qty = returned_qty + $3
			WHERE tenant_id = $1 AND id = $2
			  AND returned_qty + $3 <= quantity`,
			tenantID, line.ReceiptLineID, line.Quantity.String(),
		)
		if updateErr != nil {
			return nil, fmt.Errorf("update returned quantity: %w", updateErr)
		}
		if tag.RowsAffected() != 1 {
			return nil, domain.ErrOverReturn
		}
	}

	// Process inventory rows in a stable order to reduce deadlock risk for
	// multi-material concurrent returns.
	sortedLines := append([]*domain.PurchaseReturnLine(nil), ret.Lines...)
	sort.Slice(sortedLines, func(i, j int) bool {
		if sortedLines[i].MaterialID == sortedLines[j].MaterialID {
			return sortedLines[i].ReceiptLineID < sortedLines[j].ReceiptLineID
		}
		return sortedLines[i].MaterialID < sortedLines[j].MaterialID
	})

	for _, line := range sortedLines {
		var onHandRaw, availableRaw, avgCostRaw string
		balanceErr := tx.QueryRow(ctx, `
			SELECT on_hand::text, available::text, avg_cost::text
			FROM erp.inventory_balances
			WHERE tenant_id = $1 AND warehouse_id = $2 AND material_id = $3
			FOR UPDATE`,
			tenantID, ret.WarehouseID, line.MaterialID,
		).Scan(&onHandRaw, &availableRaw, &avgCostRaw)
		if errors.Is(balanceErr, pgx.ErrNoRows) {
			return nil, domain.ErrInsufficientStock
		}
		if balanceErr != nil {
			return nil, fmt.Errorf("lock inventory balance: %w", balanceErr)
		}

		oldOnHand := decimal.RequireFromString(onHandRaw)
		oldAvailable := decimal.RequireFromString(availableRaw)
		oldAvgCost := decimal.RequireFromString(avgCostRaw)
		if oldOnHand.LessThan(line.Quantity) || oldAvailable.LessThan(line.Quantity) {
			return nil, domain.ErrInsufficientStock
		}

		newOnHand := oldOnHand.Sub(line.Quantity)
		newAvailable := oldAvailable.Sub(line.Quantity)
		newAvgCost := decimal.Zero
		if !newOnHand.IsZero() {
			newAvgCost = oldOnHand.Mul(oldAvgCost).Sub(line.CostAmount).Div(newOnHand).Round(6)
			if newAvgCost.IsNegative() {
				newAvgCost = decimal.Zero
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE erp.inventory_balances
			SET on_hand = $4, available = $5, avg_cost = $6, updated_at = now()
			WHERE tenant_id = $1 AND warehouse_id = $2 AND material_id = $3`,
			tenantID, ret.WarehouseID, line.MaterialID,
			newOnHand.String(), newAvailable.String(), newAvgCost.String(),
		); err != nil {
			return nil, fmt.Errorf("update inventory balance: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.inventory_transactions (
				id, tenant_id, org_id, warehouse_id, material_id, direction, biz_type,
				biz_id, biz_code, quantity, unit_cost, amount, occurred_at, created_by
			) VALUES ($1,$2,$3,$4,$5,'out','purchase_return',$6,$7,$8,$9,$10,now(),$11)`,
			uuid.NewString(), tenantID, ret.OrgID, ret.WarehouseID, line.MaterialID,
			ret.ID, ret.Code, line.Quantity.String(), line.UnitCost.String(),
			line.CostAmount.String(), userID,
		); err != nil {
			return nil, fmt.Errorf("insert inventory transaction: %w", err)
		}
	}

	if err := glpost.PostPurchaseReturn(ctx, tx, glpost.PurchaseReturnPosting{
		TenantID:        tenantID,
		OrgID:           ret.OrgID,
		VoucherCode:     ret.VoucherCode,
		SourceID:        ret.ID,
		SourceCode:      ret.Code,
		SupplierID:      ret.SupplierID,
		CreatedBy:       userID,
		VoucherDate:     ret.ReturnDate,
		BaseNetAmount:   ret.BaseNetAmount,
		BaseTaxAmount:   ret.BaseTaxAmount,
		BaseGrossAmount: ret.BaseTotalAmount,
	}); err != nil {
		return nil, fmt.Errorf("post purchase return voucher: %w", err)
	}

	if ret.TotalAmount.IsPositive() && ret.BaseTotalAmount.IsPositive() {
		var invoiceID, invoicePartnerID, invoiceCurrency, invoiceRateRaw string
		var invoiceTotalRaw, invoicePaidRaw, invoiceCreditedRaw string
		var invoiceBaseTotalRaw, invoiceBasePaidRaw, invoiceBaseCreditedRaw string
		err = tx.QueryRow(ctx, `
		SELECT id::text, partner_id::text, currency, exchange_rate::text,
		       total_amount::text, paid_amount::text, credited_amount::text,
		       base_total_amount::text, base_paid_amount::text, base_credited_amount::text
		FROM erp.finance_invoices
		WHERE tenant_id = $1 AND source_type = 'purchase_receipt' AND source_id = $2
		  AND invoice_type = 'payable'
		FOR UPDATE`, tenantID, params.ReceiptID).Scan(
			&invoiceID, &invoicePartnerID, &invoiceCurrency, &invoiceRateRaw,
			&invoiceTotalRaw, &invoicePaidRaw, &invoiceCreditedRaw,
			&invoiceBaseTotalRaw, &invoiceBasePaidRaw, &invoiceBaseCreditedRaw,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("lock payable invoice: %w", err)
		}
		if invoicePartnerID != supplierID || invoiceCurrency != currency {
			return nil, domain.ErrInvalidInput
		}

		invoiceRate := decimal.RequireFromString(invoiceRateRaw)
		if !invoiceRate.Equal(ret.ExchangeRate) {
			return nil, domain.ErrInvalidInput
		}
		invoiceTotal := decimal.RequireFromString(invoiceTotalRaw)
		invoicePaid := decimal.RequireFromString(invoicePaidRaw)
		invoiceCredited := decimal.RequireFromString(invoiceCreditedRaw)
		invoiceBaseTotal := decimal.RequireFromString(invoiceBaseTotalRaw)
		invoiceBasePaid := decimal.RequireFromString(invoiceBasePaidRaw)
		invoiceBaseCredited := decimal.RequireFromString(invoiceBaseCreditedRaw)

		application := applyPurchaseReturnCredit(
			ret.TotalAmount, ret.BaseTotalAmount, invoiceRate,
			invoiceTotal, invoicePaid, invoiceCredited,
			invoiceBaseTotal, invoiceBasePaid, invoiceBaseCredited,
		)
		applied := application.Applied
		appliedBase := application.AppliedBase
		remaining := application.Remaining
		remainingBase := application.RemainingBase
		noteStatus := application.Status

		creditNoteID := uuid.NewString()
		if _, err := tx.Exec(ctx, `
		INSERT INTO erp.finance_credit_notes (
			id, tenant_id, org_id, code, credit_type, source_type, source_id, source_code,
			invoice_id, partner_id, note_date, currency, exchange_rate, amount, base_amount,
			applied_amount, applied_base_amount, remaining_amount, remaining_base_amount,
			status, reason, remark, created_by
		) VALUES ($1,$2,$3,$4,'purchase_return','purchase_return',$5,$6,$7,$8,$9,$10,$11,$12,$13,
		          $14,$15,$16,$17,$18,$19,$20,$21)`,
			creditNoteID, tenantID, ret.OrgID, params.CreditNoteCode, ret.ID, ret.Code,
			invoiceID, ret.SupplierID, ret.ReturnDate, ret.Currency, invoiceRate.String(),
			ret.TotalAmount.String(), ret.BaseTotalAmount.String(),
			applied.String(), appliedBase.String(), remaining.String(), remainingBase.String(),
			noteStatus, ret.Reason, ret.Remark, userID,
		); err != nil {
			return nil, fmt.Errorf("insert credit note: %w", mapDBError(err))
		}

		if applied.IsPositive() {
			tag, updateErr := tx.Exec(ctx, `
			UPDATE erp.finance_invoices
			SET credited_amount = credited_amount + $3,
			    base_credited_amount = base_credited_amount + $4,
			    status = CASE
			        WHEN paid_amount + credited_amount + $3 >= total_amount THEN 'paid'
			        WHEN paid_amount + credited_amount + $3 > 0 THEN 'partially_paid'
			        ELSE 'open'
			    END,
			    base_revalued_amount = CASE
			        WHEN paid_amount + credited_amount + $3 >= total_amount THEN 0
			        ELSE base_revalued_amount
			    END,
			    updated_by = $5, updated_at = now()
			WHERE tenant_id = $1 AND id = $2
			  AND paid_amount + credited_amount + $3 <= total_amount`,
				tenantID, invoiceID, applied.String(), appliedBase.String(), userID,
			)
			if updateErr != nil {
				return nil, fmt.Errorf("update invoice credited amount: %w", updateErr)
			}
			if tag.RowsAffected() != 1 {
				return nil, domain.ErrInvalidInput
			}

			if _, err := tx.Exec(ctx, `
			INSERT INTO erp.finance_credit_note_allocations (
				id, tenant_id, org_id, credit_note_id, invoice_id,
				amount, base_amount, allocation_date, created_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
				uuid.NewString(), tenantID, ret.OrgID, creditNoteID, invoiceID,
				applied.String(), appliedBase.String(), ret.ReturnDate, userID,
			); err != nil {
				return nil, fmt.Errorf("insert initial credit note allocation: %w", mapDBError(err))
			}
		}

		if _, err := tx.Exec(ctx, `
		UPDATE erp.purchase_returns
		SET credit_note_id = $3
		WHERE tenant_id = $1 AND id = $2`,
			tenantID, ret.ID, creditNoteID,
		); err != nil {
			return nil, fmt.Errorf("update purchase return credit note: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create purchase return: %w", err)
	}

	return r.Get(ctx, filter, tenantID, ret.ID)
}

type purchaseReturnCreditApplication struct {
	Applied       decimal.Decimal
	AppliedBase   decimal.Decimal
	Remaining     decimal.Decimal
	RemainingBase decimal.Decimal
	Status        string
}

// applyPurchaseReturnCredit decides how much of a purchase return credit note
// can be applied to the original payable invoice. Non-base amounts are in the
// invoice currency; *Base amounts are in the tenant base currency.
func applyPurchaseReturnCredit(
	returnTotal, returnBase, invoiceRate decimal.Decimal,
	invoiceTotal, invoicePaid, invoiceCredited decimal.Decimal,
	invoiceBaseTotal, invoiceBasePaid, invoiceBaseCredited decimal.Decimal,
) purchaseReturnCreditApplication {
	outstanding := invoiceTotal.Sub(invoicePaid).Sub(invoiceCredited)
	if outstanding.IsNegative() {
		outstanding = decimal.Zero
	}
	applied := decimal.Zero
	if outstanding.IsPositive() {
		applied = returnTotal
		if applied.GreaterThan(outstanding) {
			applied = outstanding
		}
	}

	baseOutstanding := invoiceBaseTotal.Sub(invoiceBasePaid).Sub(invoiceBaseCredited)
	if baseOutstanding.IsNegative() {
		baseOutstanding = decimal.Zero
	}
	appliedBase := applied.Mul(invoiceRate).Round(4)
	if applied.Equal(returnTotal) {
		// Full application uses the return's exact base amount and avoids
		// multiplying a rounded original-currency amount by the invoice rate.
		appliedBase = returnBase
		if baseOutstanding.LessThan(appliedBase) {
			appliedBase = baseOutstanding
		}
	} else {
		if appliedBase.GreaterThan(baseOutstanding) {
			appliedBase = baseOutstanding
		}
		if appliedBase.GreaterThan(returnBase) {
			appliedBase = returnBase
		}
	}

	remaining := returnTotal.Sub(applied).Round(4)
	remainingBase := returnBase.Sub(appliedBase).Round(4)
	status := "open"
	// A 4-decimal base residual can remain even when the original-currency
	// amount is fully applied. Keep applied/remaining base amounts consistent
	// with base_amount (the DB checks their sum) and use the tolerance only for
	// the status.
	if remaining.IsZero() && remainingBase.Abs().LessThanOrEqual(decimal.RequireFromString("0.0001")) {
		status = "applied"
	}
	return purchaseReturnCreditApplication{
		Applied:       applied,
		AppliedBase:   appliedBase,
		Remaining:     remaining,
		RemainingBase: remainingBase,
		Status:        status,
	}
}

func (r *ReturnRepository) Get(
	ctx context.Context,
	filter scope.Filter,
	tenantID, id string,
) (*domain.PurchaseReturn, error) {
	args := []any{tenantID, id}
	where := "r.tenant_id = $1 AND r.id = $2"
	appendScope(&where, &args, "r", filter)

	row := r.db.QueryRow(ctx, purchaseReturnSelectSQL+` WHERE `+where, args...)
	ret, err := scanPurchaseReturnHeader(row)
	if err != nil {
		return nil, err
	}
	linesByReturn, err := loadPurchaseReturnLinesForReturns(ctx, r.db, tenantID, []string{id})
	if err != nil {
		return nil, err
	}
	ret.Lines = linesByReturn[id]
	if ret.Lines == nil {
		ret.Lines = make([]*domain.PurchaseReturnLine, 0)
	}
	return ret, nil
}

func (r *ReturnRepository) List(
	ctx context.Context,
	filter scope.Filter,
	tenantID, receiptID, supplierID, keyword string,
	startDate, endDate time.Time,
	offset, limit int,
) ([]*domain.PurchaseReturn, int, error) {
	args := []any{tenantID}
	where := "r.tenant_id = $1"
	if receiptID != "" {
		args = append(args, receiptID)
		where += fmt.Sprintf(" AND r.receipt_id = $%d", len(args))
	}
	if supplierID != "" {
		args = append(args, supplierID)
		where += fmt.Sprintf(" AND r.supplier_id = $%d", len(args))
	}
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		index := len(args)
		where += fmt.Sprintf(" AND (r.code ILIKE $%d OR r.receipt_code ILIKE $%d OR p.name ILIKE $%d)", index, index, index)
	}
	if !startDate.IsZero() {
		args = append(args, startDate.Format("2006-01-02"))
		where += fmt.Sprintf(" AND r.return_date >= $%d::date", len(args))
	}
	if !endDate.IsZero() {
		args = append(args, endDate.Format("2006-01-02"))
		where += fmt.Sprintf(" AND r.return_date <= $%d::date", len(args))
	}
	appendScope(&where, &args, "r", filter)

	countQuery := `SELECT count(*) FROM erp.purchase_returns r
		LEFT JOIN erp.partners p ON p.tenant_id = r.tenant_id AND p.id = r.supplier_id
		WHERE ` + where
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count purchase returns: %w", err)
	}

	query := purchaseReturnSelectSQL + ` WHERE ` + where +
		fmt.Sprintf(` ORDER BY r.return_date DESC, r.code DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list purchase returns: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.PurchaseReturn, 0)
	for rows.Next() {
		item, scanErr := scanPurchaseReturnHeader(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	returnIDs := make([]string, 0, len(items))
	for _, item := range items {
		returnIDs = append(returnIDs, item.ID)
	}
	linesByReturn, err := loadPurchaseReturnLinesForReturns(ctx, r.db, tenantID, returnIDs)
	if err != nil {
		return nil, 0, err
	}
	for _, item := range items {
		item.Lines = linesByReturn[item.ID]
		if item.Lines == nil {
			item.Lines = make([]*domain.PurchaseReturnLine, 0)
		}
	}
	return items, total, nil
}

const purchaseReturnSelectSQL = `
	SELECT r.id::text, r.tenant_id::text, r.org_id::text, r.code,
	       r.receipt_id::text, r.receipt_code, r.order_id::text, r.order_code,
	       r.supplier_id::text, COALESCE(p.name, ''), r.warehouse_id::text, COALESCE(w.name, ''),
	       r.return_date, r.status, r.currency, r.exchange_rate::text, r.price_includes_tax,
	       r.net_amount::text, r.tax_amount::text, r.total_amount::text,
	       r.base_net_amount::text, r.base_tax_amount::text, r.base_total_amount::text,
	       r.cost_amount::text,
	       COALESCE(c.id::text, ''), COALESCE(c.code, ''), COALESCE(c.amount::text, '0'),
	       COALESCE(c.applied_amount::text, '0'), COALESCE(c.remaining_amount::text, '0'),
	       COALESCE(c.status, ''),
	       COALESCE(i.id::text, ''), COALESCE(i.code, ''),
	       r.voucher_code, r.reason, r.remark, r.created_by::text, r.created_at
	FROM erp.purchase_returns r
	JOIN erp.purchase_orders o ON o.tenant_id = r.tenant_id AND o.id = r.order_id
	JOIN erp.purchase_receipts rc ON rc.tenant_id = r.tenant_id AND rc.id = r.receipt_id
	LEFT JOIN erp.partners p ON p.tenant_id = r.tenant_id AND p.id = r.supplier_id
	LEFT JOIN erp.warehouses w ON w.tenant_id = r.tenant_id AND w.id = r.warehouse_id
	LEFT JOIN erp.finance_credit_notes c ON c.tenant_id = r.tenant_id AND c.id = r.credit_note_id
	LEFT JOIN erp.finance_invoices i ON i.tenant_id = r.tenant_id AND i.id = c.invoice_id`

func scanPurchaseReturnHeader(row rowScanner) (*domain.PurchaseReturn, error) {
	var item domain.PurchaseReturn
	var exchangeRate, netAmount, taxAmount, totalAmount string
	var baseNet, baseTax, baseTotal, costAmount string
	var creditAmount, creditApplied, creditRemaining string

	err := row.Scan(
		&item.ID, &item.TenantID, &item.OrgID, &item.Code,
		&item.ReceiptID, &item.ReceiptCode, &item.OrderID, &item.OrderCode,
		&item.SupplierID, &item.SupplierName, &item.WarehouseID, &item.WarehouseName,
		&item.ReturnDate, &item.Status, &item.Currency, &exchangeRate, &item.PriceIncludesTax,
		&netAmount, &taxAmount, &totalAmount,
		&baseNet, &baseTax, &baseTotal, &costAmount,
		&item.CreditNoteID, &item.CreditNoteCode, &creditAmount,
		&creditApplied, &creditRemaining, &item.CreditNoteStatus,
		&item.InvoiceID, &item.InvoiceCode,
		&item.VoucherCode, &item.Reason, &item.Remark, &item.CreatedBy, &item.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan purchase return: %w", err)
	}
	item.ExchangeRate = decimal.RequireFromString(exchangeRate)
	item.NetAmount = decimal.RequireFromString(netAmount)
	item.TaxAmount = decimal.RequireFromString(taxAmount)
	item.TotalAmount = decimal.RequireFromString(totalAmount)
	item.BaseNetAmount = decimal.RequireFromString(baseNet)
	item.BaseTaxAmount = decimal.RequireFromString(baseTax)
	item.BaseTotalAmount = decimal.RequireFromString(baseTotal)
	item.CostAmount = decimal.RequireFromString(costAmount)
	item.CreditNoteAmount = decimal.RequireFromString(creditAmount)
	item.CreditNoteApplied = decimal.RequireFromString(creditApplied)
	item.CreditNoteRemaining = decimal.RequireFromString(creditRemaining)
	return &item, nil
}

const purchaseReturnLineSelectSQL = `
	SELECT rl.id::text, rl.tenant_id::text, rl.return_id::text, rl.line_no,
	       rl.receipt_line_id::text, rl.order_line_id::text, rl.material_id::text,
	       COALESCE(m.name, ''), rl.quantity::text, rl.unit_price::text,
	       COALESCE(rl.tax_rate_id::text, ''), rl.tax_rate::text,
	       rl.amount::text, rl.tax_amount::text, rl.gross_amount::text,
	       rl.base_amount::text, rl.base_tax_amount::text, rl.base_gross_amount::text,
	       rl.unit_cost::text, rl.cost_amount::text
	FROM erp.purchase_return_lines rl
	LEFT JOIN erp.materials m ON m.tenant_id = rl.tenant_id AND m.id = rl.material_id`

func loadPurchaseReturnLinesForReturns(
	ctx context.Context,
	q querier,
	tenantID string,
	returnIDs []string,
) (map[string][]*domain.PurchaseReturnLine, error) {
	linesByReturn := make(map[string][]*domain.PurchaseReturnLine, len(returnIDs))
	if len(returnIDs) == 0 {
		return linesByReturn, nil
	}

	ids := make([]pgtype.UUID, 0, len(returnIDs))
	for _, id := range returnIDs {
		parsed, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("parse purchase return id: %w", err)
		}
		ids = append(ids, pgtype.UUID{Bytes: [16]byte(parsed), Valid: true})
	}

	rows, err := q.Query(ctx, purchaseReturnLineSelectSQL+`
		WHERE rl.tenant_id = $1 AND rl.return_id = ANY($2::uuid[])
		ORDER BY rl.return_id, rl.line_no`, tenantID, ids)
	if err != nil {
		return nil, fmt.Errorf("list purchase return lines: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		line, scanErr := scanPurchaseReturnLine(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		linesByReturn[line.ReturnID] = append(linesByReturn[line.ReturnID], line)
	}
	return linesByReturn, rows.Err()
}

func scanPurchaseReturnLine(row rowScanner) (*domain.PurchaseReturnLine, error) {
	var line domain.PurchaseReturnLine
	var quantity, unitPrice, taxRate, amount, taxAmount, grossAmount string
	var baseAmount, baseTax, baseGross, unitCost, costAmount string

	err := row.Scan(
		&line.ID, &line.TenantID, &line.ReturnID, &line.LineNo,
		&line.ReceiptLineID, &line.OrderLineID, &line.MaterialID, &line.MaterialName,
		&quantity, &unitPrice, &line.TaxRateID, &taxRate,
		&amount, &taxAmount, &grossAmount,
		&baseAmount, &baseTax, &baseGross, &unitCost, &costAmount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan purchase return line: %w", err)
	}
	line.Quantity = decimal.RequireFromString(quantity)
	line.UnitPrice = decimal.RequireFromString(unitPrice)
	line.TaxRate = decimal.RequireFromString(taxRate)
	line.Amount = decimal.RequireFromString(amount)
	line.TaxAmount = decimal.RequireFromString(taxAmount)
	line.GrossAmount = decimal.RequireFromString(grossAmount)
	line.BaseAmount = decimal.RequireFromString(baseAmount)
	line.BaseTaxAmount = decimal.RequireFromString(baseTax)
	line.BaseGrossAmount = decimal.RequireFromString(baseGross)
	line.UnitCost = decimal.RequireFromString(unitCost)
	line.CostAmount = decimal.RequireFromString(costAmount)
	return &line, nil
}
