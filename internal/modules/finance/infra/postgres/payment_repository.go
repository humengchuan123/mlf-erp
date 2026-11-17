package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/finance/domain"
	glpost "github.com/mlf-erp/mlf-erp/internal/modules/gl/infra/postgres"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type CodeGenerator interface {
	Next(ctx context.Context, tenantID, entity string) (string, error)
}

type PaymentRepository struct {
	db    *pgxpool.Pool
	codes CodeGenerator
}

func NewPaymentRepository(db *pgxpool.Pool, codes CodeGenerator) *PaymentRepository {
	return &PaymentRepository{db: db, codes: codes}
}

func (r *PaymentRepository) Create(ctx context.Context, payment *domain.Payment) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create payment: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.finance_payments (
			id, tenant_id, org_id, code, payment_type, partner_id, payment_date,
			currency, exchange_rate, amount, base_amount, allocated_amount,
			base_allocated_amount, status, method, reference, remark, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		payment.ID, payment.TenantID, payment.OrgID, payment.Code, payment.PaymentType, payment.PartnerID,
		payment.PaymentDate, payment.Currency, payment.ExchangeRate.String(),
		payment.Amount.String(), payment.BaseAmount.String(), payment.AllocatedAmount.String(),
		payment.BaseAllocatedAmount.String(), payment.Status, payment.Method,
		payment.Reference, payment.Remark, payment.CreatedBy,
	); err != nil {
		return fmt.Errorf("insert payment: %w", mapDBError(err))
	}

	if err := glpost.PostPayment(ctx, tx, glpost.PaymentPosting{
		TenantID:      payment.TenantID,
		OrgID:         payment.OrgID,
		VoucherCode:   payment.VoucherCode,
		SourceID:      payment.ID,
		SourceCode:    payment.Code,
		PartnerID:     payment.PartnerID,
		CreatedBy:     payment.CreatedBy,
		VoucherDate:   payment.PaymentDate,
		BaseAmount:    payment.BaseAmount,
		PaymentType:   payment.PaymentType,
		PaymentMethod: payment.Method,
	}); err != nil {
		return fmt.Errorf("post payment voucher: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create payment: %w", err)
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

	var paymentType, partnerID, currency, status, amountRaw, allocatedRaw, paymentRateRaw, baseAllocatedRaw string
	var paymentOrgID, paymentCode string
	var paymentDate time.Time
	err = tx.QueryRow(ctx, `
		SELECT p.payment_type, p.partner_id::text, p.currency, p.status,
		       p.amount::text, p.allocated_amount::text, p.exchange_rate::text,
		       p.base_allocated_amount::text, p.org_id::text, p.code, p.payment_date
		FROM erp.finance_payments p
		WHERE `+where+`
		FOR UPDATE OF p`, args...).Scan(
		&paymentType, &partnerID, &currency, &status, &amountRaw, &allocatedRaw,
		&paymentRateRaw, &baseAllocatedRaw, &paymentOrgID, &paymentCode, &paymentDate,
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
	paymentRate := decimal.RequireFromString(paymentRateRaw)
	baseAllocated := decimal.RequireFromString(baseAllocatedRaw)
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
		id          string
		invoiceType string
		status      string
		total       decimal.Decimal
		paid        decimal.Decimal
		rate        decimal.Decimal
		basePaid    decimal.Decimal
	}
	locked := make(map[string]lockedInvoice, len(invoiceIDs))

	for _, invoiceID := range invoiceIDs {
		allocationAmount := byInvoice[invoiceID]

		invoiceArgs := []any{tenantID, invoiceID}
		invoiceWhere := "i.tenant_id = $1 AND i.id = $2"
		appendScope(&invoiceWhere, &invoiceArgs, "i", filter)

		var invoiceType, invoicePartnerID, invoiceCurrency, invoiceStatus, invoiceTotalRaw, invoicePaidRaw string
		var invoiceRateRaw, basePaidRaw string
		err = tx.QueryRow(ctx, `
			SELECT i.invoice_type, i.partner_id::text, i.currency, i.status,
			       i.total_amount::text, i.paid_amount::text, i.exchange_rate::text,
			       i.base_paid_amount::text
			FROM erp.finance_invoices i
			WHERE `+invoiceWhere+`
			FOR UPDATE OF i`, invoiceArgs...).Scan(
			&invoiceType, &invoicePartnerID, &invoiceCurrency, &invoiceStatus,
			&invoiceTotalRaw, &invoicePaidRaw, &invoiceRateRaw, &basePaidRaw,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock invoice: %w", err)
		}

		switch paymentType {
		case domain.PaymentTypeReceipt:
			if invoiceType != domain.InvoiceTypeReceivable {
				return domain.ErrTypeMismatch
			}
		case domain.PaymentTypePayment:
			if invoiceType != domain.InvoiceTypePayable {
				return domain.ErrTypeMismatch
			}
		default:
			return domain.ErrInvalidInput
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
		invoiceRate := decimal.RequireFromString(invoiceRateRaw)
		basePaid := decimal.RequireFromString(basePaidRaw)
		outstanding := invoiceTotal.Sub(invoicePaid)
		if allocationAmount.GreaterThan(outstanding) {
			return domain.ErrOverAllocate
		}

		locked[invoiceID] = lockedInvoice{
			id: invoiceID, invoiceType: invoiceType, status: invoiceStatus,
			total: invoiceTotal, paid: invoicePaid, rate: invoiceRate, basePaid: basePaid,
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
		newBasePaid := invoice.basePaid.Add(allocationAmount.Mul(invoice.rate).Round(4))
		newStatus := domain.InvoiceStatusPartiallyPaid
		if newPaid.Equal(invoice.total) {
			newStatus = domain.InvoiceStatusPaid
		}

		tag, updateErr := tx.Exec(ctx, `
			UPDATE erp.finance_invoices
			SET paid_amount = $3, base_paid_amount = $4, status = $5,
			    base_revalued_amount = CASE WHEN $5 = 'paid' THEN 0 ELSE base_revalued_amount END,
			    updated_by = $6, updated_at = now()
			WHERE tenant_id = $1 AND id = $2 AND paid_amount + $7 <= total_amount`,
			tenantID, invoiceID, newPaid.String(), newBasePaid.String(), newStatus, userID, allocationAmount.String(),
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
		invoice := locked[invoiceID]
		basePayment := allocationAmount.Mul(paymentRate).Round(4)
		baseInvoice := allocationAmount.Mul(invoice.rate).Round(4)
		fxDiff := basePayment.Sub(baseInvoice).Round(4)
		allocationID := uuid.NewString()
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.finance_payment_allocations (
				id, tenant_id, payment_id, invoice_id, amount, base_amount,
				invoice_base_amount, created_by
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (payment_id, invoice_id) DO UPDATE
			SET amount = erp.finance_payment_allocations.amount + EXCLUDED.amount,
			    base_amount = erp.finance_payment_allocations.base_amount + EXCLUDED.base_amount,
			    invoice_base_amount = erp.finance_payment_allocations.invoice_base_amount + EXCLUDED.invoice_base_amount`,
			allocationID, tenantID, paymentID, invoiceID, allocationAmount.String(),
			basePayment.String(), baseInvoice.String(), userID,
		); err != nil {
			return fmt.Errorf("insert payment allocation: %w", mapDBError(err))
		}
		if fxDiff.IsZero() {
			continue
		}
		settlementID := uuid.NewString()
		voucherCode, codeErr := r.codes.Next(ctx, tenantID, "gl_voucher")
		if codeErr != nil {
			return codeErr
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.finance_fx_settlements (
				id, tenant_id, payment_id, invoice_id, amount, payment_rate,
				invoice_rate, base_adjustment, voucher_id, created_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			settlementID, tenantID, paymentID, invoiceID, allocationAmount.String(),
			paymentRate.String(), invoice.rate.String(), fxDiff.String(), nil, userID,
		); err != nil {
			return fmt.Errorf("insert FX settlement: %w", mapDBError(err))
		}
		if err := glpost.PostFXSettlement(ctx, tx, glpost.FXSettlementPosting{
			TenantID:       tenantID,
			OrgID:          paymentOrgID,
			VoucherCode:    voucherCode,
			SourceID:       settlementID,
			SourceCode:     paymentCode,
			PartnerID:      partnerID,
			CreatedBy:      userID,
			VoucherDate:    paymentDate,
			InvoiceType:    invoice.invoiceType,
			BaseAdjustment: fxDiff,
		}); err != nil {
			return fmt.Errorf("post FX settlement voucher: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE erp.finance_fx_settlements s
			SET voucher_id = v.id
			FROM erp.gl_vouchers v
			WHERE s.tenant_id = $1 AND s.id = $2
			  AND v.tenant_id = s.tenant_id AND v.source_type = 'fx_settlement' AND v.source_id = s.id`,
			tenantID, settlementID,
		); err != nil {
			return fmt.Errorf("link FX settlement voucher: %w", err)
		}
	}

	newAllocated := allocated.Add(total)
	newBaseAllocated := baseAllocated.Add(total.Mul(paymentRate).Round(4))
	newStatus := domain.PaymentStatusPartiallyAllocated
	if newAllocated.Equal(amount) {
		newStatus = domain.PaymentStatusAllocated
	}
	tag, err := tx.Exec(ctx, `
		UPDATE erp.finance_payments
		SET allocated_amount = $3, base_allocated_amount = $4, status = $5
		WHERE tenant_id = $1 AND id = $2 AND allocated_amount + $6 <= amount`,
		tenantID, paymentID, newAllocated.String(), newBaseAllocated.String(),
		newStatus, total.String(),
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
	       p.exchange_rate::text, p.amount::text, p.base_amount::text,
	       p.allocated_amount::text, p.base_allocated_amount::text, p.status,
	       p.method, p.reference, p.remark,
	       p.created_by::text, p.created_at
	FROM erp.finance_payments p
	LEFT JOIN erp.partners pt ON pt.tenant_id = p.tenant_id AND pt.id = p.partner_id`

func scanPayment(row rowScanner) (*domain.Payment, error) {
	var item domain.Payment
	var exchangeRate, amount, baseAmount, allocated, baseAllocated string
	err := row.Scan(
		&item.ID, &item.TenantID, &item.OrgID, &item.Code, &item.PaymentType,
		&item.PartnerID, &item.PartnerName, &item.PaymentDate, &item.Currency,
		&exchangeRate, &amount, &baseAmount, &allocated, &baseAllocated, &item.Status,
		&item.Method, &item.Reference, &item.Remark,
		&item.CreatedBy, &item.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan payment: %w", err)
	}
	item.ExchangeRate = decimal.RequireFromString(exchangeRate)
	item.Amount = decimal.RequireFromString(amount)
	item.BaseAmount = decimal.RequireFromString(baseAmount)
	item.AllocatedAmount = decimal.RequireFromString(allocated)
	item.BaseAllocatedAmount = decimal.RequireFromString(baseAllocated)
	return &item, nil
}
