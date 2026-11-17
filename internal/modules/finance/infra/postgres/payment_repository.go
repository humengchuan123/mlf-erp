package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/finance/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type PaymentRepository struct {
	db *pgxpool.Pool
}

func NewPaymentRepository(db *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(ctx context.Context, payment *domain.Payment) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO erp.finance_payments (
			id, tenant_id, org_id, code, payment_type, partner_id, payment_date,
			currency, amount, allocated_amount, status, method, reference, remark, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		payment.ID, payment.TenantID, payment.OrgID, payment.Code, payment.PaymentType, payment.PartnerID,
		payment.PaymentDate, payment.Currency, payment.Amount.String(), payment.AllocatedAmount.String(),
		payment.Status, payment.Method, payment.Reference, payment.Remark, payment.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("insert payment: %w", mapDBError(err))
	}
	return nil
}

func (r *PaymentRepository) Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*domain.Payment, error) {
	args := []any{tenantID, id}
	where := "p.tenant_id = $1 AND p.id = $2"
	appendScope(&where, &args, "p", filter)

	row := r.db.QueryRow(ctx, paymentSelectSQL+` WHERE `+where, args...)
	return scanPayment(row)
}

func (r *PaymentRepository) List(
	ctx context.Context,
	filter scope.Filter,
	tenantID, paymentType, status, partnerID, keyword string,
	offset, limit int,
) ([]*domain.Payment, int, error) {
	args := []any{tenantID}
	where := "p.tenant_id = $1"
	if paymentType != "" {
		args = append(args, paymentType)
		where += fmt.Sprintf(" AND p.payment_type = $%d", len(args))
	}
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND p.status = $%d", len(args))
	}
	if partnerID != "" {
		args = append(args, partnerID)
		where += fmt.Sprintf(" AND p.partner_id = $%d", len(args))
	}
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		index := len(args)
		where += fmt.Sprintf(" AND (p.code ILIKE $%d OR p.reference ILIKE $%d OR pt.name ILIKE $%d)", index, index, index)
	}
	appendScope(&where, &args, "p", filter)

	countQuery := `SELECT count(*) FROM erp.finance_payments p
		LEFT JOIN erp.partners pt ON pt.tenant_id = p.tenant_id AND pt.id = p.partner_id
		WHERE ` + where
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count payments: %w", err)
	}

	query := paymentSelectSQL + ` WHERE ` + where +
		fmt.Sprintf(` ORDER BY p.payment_date DESC, p.created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Payment, 0)
	for rows.Next() {
		item, scanErr := scanPayment(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *PaymentRepository) Allocate(
	ctx context.Context,
	filter scope.Filter,
	tenantID, paymentID, userID string,
	allocations []*domain.AllocationInput,
) error {
	if len(allocations) == 0 {
		return domain.ErrInvalidInput
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin allocate payment: %w", err)
	}
	defer tx.Rollback(ctx)

	args := []any{tenantID, paymentID}
	where := "p.tenant_id = $1 AND p.id = $2"
	appendScope(&where, &args, "p", filter)

	var paymentType, partnerID, currency, status, amountRaw, allocatedRaw string
	err = tx.QueryRow(ctx, `
		SELECT p.payment_type, p.partner_id::text, p.currency, p.status,
		       p.amount::text, p.allocated_amount::text
		FROM erp.finance_payments p
		WHERE `+where+`
		FOR UPDATE OF p`, args...).Scan(
		&paymentType, &partnerID, &currency, &status, &amountRaw, &allocatedRaw,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock payment: %w", err)
	}
	if status == domain.PaymentStatusAllocated {
		return domain.ErrPaymentAllocated
	}

	amount := decimal.RequireFromString(amountRaw)
	allocated := decimal.RequireFromString(allocatedRaw)
	unallocated := amount.Sub(allocated)
	if !unallocated.IsPositive() {
		return domain.ErrPaymentAllocated
	}

	seen := make(map[string]struct{}, len(allocations))
	invoiceIDs := make([]string, 0, len(allocations))
	byInvoice := make(map[string]decimal.Decimal, len(allocations))
	for _, allocation := range allocations {
		if allocation == nil || allocation.InvoiceID == "" || !allocation.Amount.IsPositive() {
			return domain.ErrInvalidInput
		}
		if _, exists := seen[allocation.InvoiceID]; exists {
			return domain.ErrInvalidInput
		}
		seen[allocation.InvoiceID] = struct{}{}
		invoiceIDs = append(invoiceIDs, allocation.InvoiceID)
		byInvoice[allocation.InvoiceID] = allocation.Amount
	}
	sort.Strings(invoiceIDs)

	total := decimal.Zero
	type lockedInvoice struct {
		id     string
		status string
		total  decimal.Decimal
		paid   decimal.Decimal
	}
	locked := make(map[string]lockedInvoice, len(invoiceIDs))

	for _, invoiceID := range invoiceIDs {
		allocationAmount := byInvoice[invoiceID]

		invoiceArgs := []any{tenantID, invoiceID}
		invoiceWhere := "i.tenant_id = $1 AND i.id = $2"
		appendScope(&invoiceWhere, &invoiceArgs, "i", filter)

		var invoiceType, invoicePartnerID, invoiceCurrency, invoiceStatus, invoiceTotalRaw, invoicePaidRaw string
		err = tx.QueryRow(ctx, `
			SELECT i.invoice_type, i.partner_id::text, i.currency, i.status,
			       i.total_amount::text, i.paid_amount::text
			FROM erp.finance_invoices i
			WHERE `+invoiceWhere+`
			FOR UPDATE OF i`, invoiceArgs...).Scan(
			&invoiceType, &invoicePartnerID, &invoiceCurrency, &invoiceStatus,
			&invoiceTotalRaw, &invoicePaidRaw,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock invoice: %w", err)
		}

		if paymentType == domain.PaymentTypeReceipt && invoiceType != domain.InvoiceTypeReceivable {
			return domain.ErrTypeMismatch
		}
		if paymentType == domain.PaymentTypePayment && invoiceType != domain.InvoiceTypePayable {
			return domain.ErrTypeMismatch
		}
		if invoicePartnerID != partnerID {
			return domain.ErrPartnerMismatch
		}
		if invoiceCurrency != currency {
			return domain.ErrCurrencyMismatch
		}
		if invoiceStatus == domain.InvoiceStatusPaid {
			return domain.ErrInvoicePaid
		}

		invoiceTotal := decimal.RequireFromString(invoiceTotalRaw)
		invoicePaid := decimal.RequireFromString(invoicePaidRaw)
		outstanding := invoiceTotal.Sub(invoicePaid)
		if allocationAmount.GreaterThan(outstanding) {
			return domain.ErrOverAllocate
		}

		locked[invoiceID] = lockedInvoice{
			id: invoiceID, status: invoiceStatus, total: invoiceTotal, paid: invoicePaid,
		}
		total = total.Add(allocationAmount)
	}

	if total.GreaterThan(unallocated) {
		return domain.ErrOverAllocate
	}

	for _, invoiceID := range invoiceIDs {
		allocationAmount := byInvoice[invoiceID]
		invoice := locked[invoiceID]
		newPaid := invoice.paid.Add(allocationAmount)
		newStatus := domain.InvoiceStatusPartiallyPaid
		if newPaid.Equal(invoice.total) {
			newStatus = domain.InvoiceStatusPaid
		}

		tag, updateErr := tx.Exec(ctx, `
			UPDATE erp.finance_invoices
			SET paid_amount = $3, status = $4, updated_by = $5, updated_at = now()
			WHERE tenant_id = $1 AND id = $2 AND paid_amount + $6 <= total_amount`,
			tenantID, invoiceID, newPaid.String(), newStatus, userID, allocationAmount.String(),
		)
		if updateErr != nil {
			return fmt.Errorf("update invoice paid amount: %w", updateErr)
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrOverAllocate
		}
	}

	for _, invoiceID := range invoiceIDs {
		allocationAmount := byInvoice[invoiceID]
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.finance_payment_allocations (
				id, tenant_id, payment_id, invoice_id, amount, created_by
			) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)`,
			tenantID, paymentID, invoiceID, allocationAmount.String(), userID,
		); err != nil {
			return fmt.Errorf("insert payment allocation: %w", mapDBError(err))
		}
	}

	newAllocated := allocated.Add(total)
	newStatus := domain.PaymentStatusPartiallyAllocated
	if newAllocated.Equal(amount) {
		newStatus = domain.PaymentStatusAllocated
	}
	tag, err := tx.Exec(ctx, `
		UPDATE erp.finance_payments
		SET allocated_amount = $3, status = $4
		WHERE tenant_id = $1 AND id = $2 AND allocated_amount + $5 <= amount`,
		tenantID, paymentID, newAllocated.String(), newStatus, total.String(),
	)
	if err != nil {
		return fmt.Errorf("update payment allocated amount: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrOverAllocate
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit allocate payment: %w", err)
	}
	return nil
}

const paymentSelectSQL = `
	SELECT p.id::text, p.tenant_id::text, p.org_id::text, p.code, p.payment_type,
	       p.partner_id::text, COALESCE(pt.name, ''), p.payment_date, p.currency,
	       p.amount::text, p.allocated_amount::text, p.status, p.method, p.reference, p.remark,
	       p.created_by::text, p.created_at
	FROM erp.finance_payments p
	LEFT JOIN erp.partners pt ON pt.tenant_id = p.tenant_id AND pt.id = p.partner_id`

func scanPayment(row rowScanner) (*domain.Payment, error) {
	var item domain.Payment
	var amount, allocated string
	err := row.Scan(
		&item.ID, &item.TenantID, &item.OrgID, &item.Code, &item.PaymentType,
		&item.PartnerID, &item.PartnerName, &item.PaymentDate, &item.Currency,
		&amount, &allocated, &item.Status, &item.Method, &item.Reference, &item.Remark,
		&item.CreatedBy, &item.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan payment: %w", err)
	}
	item.Amount = decimal.RequireFromString(amount)
	item.AllocatedAmount = decimal.RequireFromString(allocated)
	return &item, nil
}
