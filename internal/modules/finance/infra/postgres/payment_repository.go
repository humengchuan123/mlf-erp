package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
		parsed, err := uuid.Parse(allocation.InvoiceID)
		if err != nil {
			return domain.ErrInvalidInput
		}
		invoiceID := parsed.String()
		if _, exists := seen[invoiceID]; exists {
			return domain.ErrInvalidInput
		}
		seen[invoiceID] = struct{}{}
		invoiceIDs = append(invoiceIDs, invoiceID)
		byInvoice[invoiceID] = allocation.Amount
	}
	sort.Strings(invoiceIDs)

	total := decimal.Zero
	type lockedInvoice struct {
		id          string
		invoiceType string
		status      string
		total       decimal.Decimal
		paid        decimal.Decimal
		credited    decimal.Decimal
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
		var invoiceRateRaw, basePaidRaw, invoiceCreditedRaw string
		err = tx.QueryRow(ctx, `
			SELECT i.invoice_type, i.partner_id::text, i.currency, i.status,
			       i.total_amount::text, i.paid_amount::text, i.exchange_rate::text,
			       i.base_paid_amount::text, i.credited_amount::text
			FROM erp.finance_invoices i
			WHERE `+invoiceWhere+`
			FOR UPDATE OF i`, invoiceArgs...).Scan(
			&invoiceType, &invoicePartnerID, &invoiceCurrency, &invoiceStatus,
			&invoiceTotalRaw, &invoicePaidRaw, &invoiceRateRaw, &basePaidRaw, &invoiceCreditedRaw,
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
		invoiceCredited := decimal.RequireFromString(invoiceCreditedRaw)
		invoiceRate := decimal.RequireFromString(invoiceRateRaw)
		basePaid := decimal.RequireFromString(basePaidRaw)
		outstanding := invoiceTotal.Sub(invoicePaid).Sub(invoiceCredited)
		if allocationAmount.GreaterThan(outstanding) {
			return domain.ErrOverAllocate
		}

		locked[invoiceID] = lockedInvoice{
			id: invoiceID, invoiceType: invoiceType, status: invoiceStatus,
			total: invoiceTotal, paid: invoicePaid, credited: invoiceCredited,
			rate: invoiceRate, basePaid: basePaid,
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
		newStatus := domain.InvoiceStatusAfterSettlement(invoice.total, newPaid, invoice.credited)

		tag, updateErr := tx.Exec(ctx, `
			UPDATE erp.finance_invoices
			SET paid_amount = $3, base_paid_amount = $4, status = $5,
			    base_revalued_amount = CASE WHEN $5 = 'paid' THEN 0 ELSE base_revalued_amount END,
			    updated_by = $6, updated_at = now()
			WHERE tenant_id = $1 AND id = $2 AND paid_amount + credited_amount + $7 <= total_amount`,
			tenantID, invoiceID, newPaid.String(), newBasePaid.String(), newStatus, userID, allocationAmount.String(),
		)
		if updateErr != nil {
			return fmt.Errorf("update invoice paid amount: %w", updateErr)
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrOverAllocate
		}
	}

	basePaymentTotal := decimal.Zero
	for _, invoiceID := range invoiceIDs {
		allocationAmount := byInvoice[invoiceID]
		invoice := locked[invoiceID]
		basePayment := allocationAmount.Mul(paymentRate).Round(4)
		basePaymentTotal = basePaymentTotal.Add(basePayment)
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
	newBaseAllocated := baseAllocated.Add(basePaymentTotal)
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

// Unallocate reverses whole payment allocation lines for the selected
// invoices. It deletes the active allocation rows, records an audit reversal
// per allocation, restores payment/invoice balances and statuses, and posts
// reversing vouchers for any posted FX settlements of those allocations.
func (r *PaymentRepository) Unallocate(
	ctx context.Context,
	filter scope.Filter,
	tenantID, paymentID, userID string,
	input domain.UnallocateInput,
) (*domain.Payment, error) {
	if len(input.InvoiceIDs) == 0 {
		return nil, domain.ErrInvalidInput
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return nil, domain.ErrReasonRequired
	}

	seen := make(map[string]struct{}, len(input.InvoiceIDs))
	invoiceIDs := make([]string, 0, len(input.InvoiceIDs))
	for _, raw := range input.InvoiceIDs {
		parsed, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, domain.ErrInvalidInput
		}
		invoiceID := parsed.String()
		if _, exists := seen[invoiceID]; exists {
			return nil, domain.ErrDuplicateInvoice
		}
		seen[invoiceID] = struct{}{}
		invoiceIDs = append(invoiceIDs, invoiceID)
	}
	sort.Strings(invoiceIDs)

	invoiceUUIDs := make([]pgtype.UUID, len(invoiceIDs))
	for i, invoiceID := range invoiceIDs {
		invoiceUUIDs[i] = pgtype.UUID{Bytes: uuid.MustParse(invoiceID), Valid: true}
	}

	reversalDate := input.ReversalDate
	if reversalDate.IsZero() {
		reversalDate = time.Now()
	}
	reversalDate = dateOnlyUTC(reversalDate)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin unallocate payment: %w", err)
	}
	defer tx.Rollback(ctx)

	args := []any{tenantID, paymentID}
	where := "p.tenant_id = $1 AND p.id = $2"
	appendScope(&where, &args, "p", filter)

	var paymentType, partnerID, currency, status, orgID, paymentCode string
	var paymentDate time.Time
	var amountRaw, allocatedRaw, baseAllocatedRaw string
	err = tx.QueryRow(ctx, `
		SELECT p.payment_type, p.partner_id::text, p.currency, p.status,
		       p.org_id::text, p.code, p.payment_date,
		       p.amount::text, p.allocated_amount::text, p.base_allocated_amount::text
		FROM erp.finance_payments p
		WHERE `+where+`
		FOR UPDATE OF p`, args...).Scan(
		&paymentType, &partnerID, &currency, &status, &orgID, &paymentCode,
		&paymentDate, &amountRaw, &allocatedRaw, &baseAllocatedRaw,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock payment: %w", err)
	}
	if status != domain.PaymentStatusAllocated && status != domain.PaymentStatusPartiallyAllocated {
		return nil, domain.ErrPaymentNotAllocated
	}
	paymentDate = dateOnlyUTC(paymentDate)
	if reversalDate.Before(paymentDate) {
		return nil, domain.ErrInvalidInput
	}

	amount := decimal.RequireFromString(amountRaw)
	allocated := decimal.RequireFromString(allocatedRaw)
	baseAllocated := decimal.RequireFromString(baseAllocatedRaw)

	type lockedInvoice struct {
		invoiceType string
		total       decimal.Decimal
		paid        decimal.Decimal
		basePaid    decimal.Decimal
		credited    decimal.Decimal
	}
	locked := make(map[string]lockedInvoice, len(invoiceIDs))

	for _, invoiceID := range invoiceIDs {
		invoiceArgs := []any{tenantID, invoiceID}
		invoiceWhere := "i.tenant_id = $1 AND i.id = $2"
		appendScope(&invoiceWhere, &invoiceArgs, "i", filter)

		var invoiceType, invoicePartnerID, invoiceCurrency string
		var totalRaw, paidRaw, basePaidRaw, creditedRaw string
		err = tx.QueryRow(ctx, `
			SELECT i.invoice_type, i.partner_id::text, i.currency,
			       i.total_amount::text, i.paid_amount::text,
			       i.base_paid_amount::text, i.credited_amount::text
			FROM erp.finance_invoices i
			WHERE `+invoiceWhere+`
			FOR UPDATE OF i`, invoiceArgs...).Scan(
			&invoiceType, &invoicePartnerID, &invoiceCurrency,
			&totalRaw, &paidRaw, &basePaidRaw, &creditedRaw,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("lock invoice: %w", err)
		}

		switch paymentType {
		case domain.PaymentTypeReceipt:
			if invoiceType != domain.InvoiceTypeReceivable {
				return nil, domain.ErrTypeMismatch
			}
		case domain.PaymentTypePayment:
			if invoiceType != domain.InvoiceTypePayable {
				return nil, domain.ErrTypeMismatch
			}
		default:
			return nil, domain.ErrInvalidInput
		}
		if invoicePartnerID != partnerID {
			return nil, domain.ErrPartnerMismatch
		}
		if invoiceCurrency != currency {
			return nil, domain.ErrCurrencyMismatch
		}

		locked[invoiceID] = lockedInvoice{
			invoiceType: invoiceType,
			total:       decimal.RequireFromString(totalRaw),
			paid:        decimal.RequireFromString(paidRaw),
			basePaid:    decimal.RequireFromString(basePaidRaw),
			credited:    decimal.RequireFromString(creditedRaw),
		}
	}

	type allocationRow struct {
		id                string
		invoiceID         string
		amount            decimal.Decimal
		baseAmount        decimal.Decimal
		invoiceBaseAmount decimal.Decimal
	}

	rows, err := tx.Query(ctx, `
		SELECT id::text, invoice_id::text, amount::text, base_amount::text,
		       invoice_base_amount::text
		FROM erp.finance_payment_allocations
		WHERE tenant_id = $1 AND payment_id = $2 AND invoice_id = ANY($3::uuid[])
		FOR UPDATE`, tenantID, paymentID, invoiceUUIDs)
	if err != nil {
		return nil, fmt.Errorf("lock payment allocations: %w", err)
	}
	defer rows.Close()

	allocationByInvoice := make(map[string]allocationRow, len(invoiceIDs))
	total := decimal.Zero
	totalBase := decimal.Zero
	for rows.Next() {
		var item allocationRow
		var amountRaw, baseRaw, invoiceBaseRaw string
		if err := rows.Scan(&item.id, &item.invoiceID, &amountRaw, &baseRaw, &invoiceBaseRaw); err != nil {
			return nil, fmt.Errorf("scan payment allocation: %w", err)
		}
		item.amount = decimal.RequireFromString(amountRaw)
		item.baseAmount = decimal.RequireFromString(baseRaw)
		item.invoiceBaseAmount = decimal.RequireFromString(invoiceBaseRaw)
		allocationByInvoice[item.invoiceID] = item
		total = total.Add(item.amount)
		totalBase = totalBase.Add(item.baseAmount)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate payment allocations: %w", err)
	}
	rows.Close()
	for _, invoiceID := range invoiceIDs {
		if _, ok := allocationByInvoice[invoiceID]; !ok {
			return nil, domain.ErrAllocationNotFound
		}
	}

	if total.GreaterThan(allocated) || totalBase.GreaterThan(baseAllocated) {
		return nil, domain.ErrPaymentNotAllocated
	}
	for _, invoiceID := range invoiceIDs {
		allocation := allocationByInvoice[invoiceID]
		invoice := locked[invoiceID]
		if allocation.amount.GreaterThan(invoice.paid) ||
			allocation.invoiceBaseAmount.GreaterThan(invoice.basePaid) {
			return nil, domain.ErrAllocationNotFound
		}
	}

	reversalByInvoice := make(map[string]string, len(invoiceIDs))
	allocationIDs := make([]pgtype.UUID, 0, len(allocationByInvoice))
	for _, invoiceID := range invoiceIDs {
		allocation := allocationByInvoice[invoiceID]
		reversalID := uuid.NewString()
		reversalByInvoice[invoiceID] = reversalID
		allocationIDs = append(allocationIDs, pgtype.UUID{Bytes: uuid.MustParse(allocation.id), Valid: true})
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.finance_payment_allocation_reversals (
				id, tenant_id, org_id, payment_id, invoice_id, allocation_id,
				amount, base_amount, invoice_base_amount, reversal_date, reason, created_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			reversalID, tenantID, orgID, paymentID, invoiceID, allocation.id,
			allocation.amount.String(), allocation.baseAmount.String(),
			allocation.invoiceBaseAmount.String(), reversalDate, reason, userID,
		); err != nil {
			return nil, fmt.Errorf("insert payment allocation reversal: %w", mapDBError(err))
		}
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM erp.finance_payment_allocations
		WHERE tenant_id = $1 AND payment_id = $2 AND id = ANY($3::uuid[])`,
		tenantID, paymentID, allocationIDs,
	); err != nil {
		return nil, fmt.Errorf("delete payment allocations: %w", err)
	}

	newAllocated := allocated.Sub(total)
	newStatus := domain.PaymentStatusAfterReversal(amount, newAllocated)
	tag, err := tx.Exec(ctx, `
		UPDATE erp.finance_payments
		SET allocated_amount = allocated_amount - $3,
		    base_allocated_amount = base_allocated_amount - $4,
		    status = $5
		WHERE tenant_id = $1 AND id = $2 AND allocated_amount >= $3
		  AND base_allocated_amount >= $4`,
		tenantID, paymentID, total.String(), totalBase.String(), newStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("update payment after reversal: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return nil, domain.ErrPaymentNotAllocated
	}

	for _, invoiceID := range invoiceIDs {
		allocation := allocationByInvoice[invoiceID]
		invoice := locked[invoiceID]
		newPaid := invoice.paid.Sub(allocation.amount)
		newInvoiceStatus := domain.InvoiceStatusAfterSettlement(invoice.total, newPaid, invoice.credited)
		tag, err := tx.Exec(ctx, `
			UPDATE erp.finance_invoices
			SET paid_amount = paid_amount - $3,
			    base_paid_amount = base_paid_amount - $4,
			    status = $5,
			    base_revalued_amount = CASE WHEN $5 = 'paid' THEN 0 ELSE base_revalued_amount END,
			    updated_by = $6, updated_at = now()
			WHERE tenant_id = $1 AND id = $2 AND paid_amount >= $3 AND base_paid_amount >= $4`,
			tenantID, invoiceID, allocation.amount.String(), allocation.invoiceBaseAmount.String(),
			newInvoiceStatus, userID,
		)
		if err != nil {
			return nil, fmt.Errorf("update invoice after reversal: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return nil, domain.ErrAllocationNotFound
		}
	}

	for _, invoiceID := range invoiceIDs {
		invoice := locked[invoiceID]
		settlementRows, err := tx.Query(ctx, `
			SELECT id::text, base_adjustment::text
			FROM erp.finance_fx_settlements
			WHERE tenant_id = $1 AND payment_id = $2 AND invoice_id = $3 AND status = 'posted'
			FOR UPDATE`, tenantID, paymentID, invoiceID)
		if err != nil {
			return nil, fmt.Errorf("lock FX settlements: %w", err)
		}

		type settlement struct {
			id             string
			baseAdjustment decimal.Decimal
		}
		settlements := make([]settlement, 0)
		for settlementRows.Next() {
			var item settlement
			var baseAdjustmentRaw string
			if err := settlementRows.Scan(&item.id, &baseAdjustmentRaw); err != nil {
				settlementRows.Close()
				return nil, fmt.Errorf("scan FX settlement: %w", err)
			}
			item.baseAdjustment = decimal.RequireFromString(baseAdjustmentRaw)
			settlements = append(settlements, item)
		}
		if err := settlementRows.Err(); err != nil {
			settlementRows.Close()
			return nil, fmt.Errorf("iterate FX settlements: %w", err)
		}
		settlementRows.Close()

		for _, item := range settlements {
			reversalID := reversalByInvoice[invoiceID]
			if item.baseAdjustment.Round(4).IsZero() {
				tag, err := tx.Exec(ctx, `
					UPDATE erp.finance_fx_settlements
					SET status = 'reversed', reversal_id = $3, reversal_voucher_id = NULL,
					    reversed_at = now(), reversed_by = $4
					WHERE tenant_id = $1 AND id = $2 AND status = 'posted'`,
					tenantID, item.id, reversalID, userID,
				)
				if err != nil {
					return nil, fmt.Errorf("mark FX settlement reversed: %w", mapDBError(err))
				}
				if tag.RowsAffected() != 1 {
					return nil, fmt.Errorf("mark FX settlement reversed: expected 1 row, got %d", tag.RowsAffected())
				}
				continue
			}
			voucherCode, err := r.codes.Next(ctx, tenantID, "gl_voucher")
			if err != nil {
				return nil, err
			}
			if err := glpost.PostFXSettlementReversal(ctx, tx, glpost.FXSettlementReversalPosting{
				TenantID:       tenantID,
				OrgID:          orgID,
				VoucherCode:    voucherCode,
				SourceID:       item.id,
				SourceCode:     paymentCode,
				InvoiceType:    invoice.invoiceType,
				PartnerID:      partnerID,
				VoucherDate:    reversalDate,
				BaseAdjustment: item.baseAdjustment,
				CreatedBy:      userID,
			}); err != nil {
				return nil, fmt.Errorf("post FX settlement reversal voucher: %w", err)
			}
			tag, err := tx.Exec(ctx, `
				UPDATE erp.finance_fx_settlements s
				SET status = 'reversed', reversal_id = $3, reversal_voucher_id = v.id,
				    reversed_at = now(), reversed_by = $4
				FROM erp.gl_vouchers v
				WHERE s.tenant_id = $1 AND s.id = $2 AND s.status = 'posted'
				  AND v.tenant_id = s.tenant_id
				  AND v.source_type = 'fx_settlement_reversal' AND v.source_id = s.id`,
				tenantID, item.id, reversalID, userID,
			)
			if err != nil {
				return nil, fmt.Errorf("mark FX settlement reversed: %w", mapDBError(err))
			}
			if tag.RowsAffected() != 1 {
				return nil, fmt.Errorf("mark FX settlement reversed: expected 1 row, got %d", tag.RowsAffected())
			}
		}
	}

	if err := glpost.CheckPeriodOpen(ctx, tx, tenantID, reversalDate); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit unallocate payment: %w", err)
	}
	return r.Get(ctx, filter, tenantID, paymentID)
}

func (r *PaymentRepository) ListAllocations(ctx context.Context, filter scope.Filter, tenantID, paymentID string) ([]*domain.PaymentAllocation, error) {
	args := []any{tenantID, paymentID}
	where := "a.tenant_id = $1 AND a.payment_id = $2"
	// Scope on the parent payment: the payment itself was already authorized
	// for read, so child allocation rows must not additionally be filtered by
	// the invoice or its creator.
	appendScope(&where, &args, "p", filter)

	rows, err := r.db.Query(ctx, `
		SELECT a.id::text, a.tenant_id::text, a.payment_id::text, a.invoice_id::text,
		       i.code, i.invoice_date, a.amount::text, a.base_amount::text,
		       a.invoice_base_amount::text, a.created_by::text, a.created_at
		FROM erp.finance_payment_allocations a
		JOIN erp.finance_payments p ON p.tenant_id = a.tenant_id AND p.id = a.payment_id
		JOIN erp.finance_invoices i ON i.tenant_id = a.tenant_id AND i.id = a.invoice_id
		WHERE `+where+`
		ORDER BY i.invoice_date, i.code`, args...)
	if err != nil {
		return nil, fmt.Errorf("list payment allocations: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.PaymentAllocation, 0)
	for rows.Next() {
		item := &domain.PaymentAllocation{}
		var amount, baseAmount, invoiceBaseAmount string
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.PaymentID, &item.InvoiceID,
			&item.InvoiceCode, &item.InvoiceDate, &amount, &baseAmount,
			&invoiceBaseAmount, &item.CreatedBy, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan payment allocation: %w", err)
		}
		item.Amount = decimal.RequireFromString(amount)
		item.BaseAmount = decimal.RequireFromString(baseAmount)
		item.InvoiceBaseAmount = decimal.RequireFromString(invoiceBaseAmount)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PaymentRepository) ListReversals(ctx context.Context, filter scope.Filter, tenantID, paymentID string) ([]*domain.PaymentAllocationReversal, error) {
	args := []any{tenantID, paymentID}
	where := "r.tenant_id = $1 AND r.payment_id = $2"
	// Scope on the parent payment: the payment itself was already authorized
	// for read, so child reversal rows must not additionally be filtered by
	// the invoice or its creator.
	appendScope(&where, &args, "p", filter)

	rows, err := r.db.Query(ctx, paymentAllocationReversalSelectSQL+`
		WHERE `+where+`
		ORDER BY r.reversal_date DESC, r.created_at DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("list payment allocation reversals: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.PaymentAllocationReversal, 0)
	for rows.Next() {
		item, scanErr := scanPaymentAllocationReversal(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

const paymentAllocationReversalSelectSQL = `
	SELECT r.id::text, r.tenant_id::text, r.org_id::text,
	       r.payment_id::text, p.code, p.payment_date,
	       r.invoice_id::text, i.code, i.invoice_date,
	       r.allocation_id::text, r.amount::text, r.base_amount::text,
	       r.invoice_base_amount::text, r.reversal_date, r.reason,
	       COALESCE(fx.voucher_codes, ''), r.created_by::text, r.created_at
	FROM erp.finance_payment_allocation_reversals r
	JOIN erp.finance_payments p ON p.tenant_id = r.tenant_id AND p.id = r.payment_id
	JOIN erp.finance_invoices i ON i.tenant_id = r.tenant_id AND i.id = r.invoice_id
	LEFT JOIN (
		SELECT s.tenant_id, s.reversal_id,
		       string_agg(DISTINCT v.code, ',' ORDER BY v.code) AS voucher_codes
		FROM erp.finance_fx_settlements s
		JOIN erp.gl_vouchers v ON v.tenant_id = s.tenant_id AND v.id = s.reversal_voucher_id
		WHERE s.reversal_id IS NOT NULL
		GROUP BY s.tenant_id, s.reversal_id
	) fx ON fx.tenant_id = r.tenant_id AND fx.reversal_id = r.id`

func scanPaymentAllocationReversal(row rowScanner) (*domain.PaymentAllocationReversal, error) {
	item := &domain.PaymentAllocationReversal{}
	var amount, baseAmount, invoiceBaseAmount, voucherCodes string
	err := row.Scan(
		&item.ID, &item.TenantID, &item.OrgID,
		&item.PaymentID, &item.PaymentCode, &item.PaymentDate,
		&item.InvoiceID, &item.InvoiceCode, &item.InvoiceDate,
		&item.AllocationID, &amount, &baseAmount, &invoiceBaseAmount,
		&item.ReversalDate, &item.Reason, &voucherCodes,
		&item.CreatedBy, &item.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan payment allocation reversal: %w", err)
	}
	item.Amount = decimal.RequireFromString(amount)
	item.BaseAmount = decimal.RequireFromString(baseAmount)
	item.InvoiceBaseAmount = decimal.RequireFromString(invoiceBaseAmount)
	item.FXReversalVoucherCodes = splitVoucherCodes(voucherCodes)
	return item, nil
}

func splitVoucherCodes(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	codes := make([]string, 0, len(parts))
	for _, part := range parts {
		if code := strings.TrimSpace(part); code != "" {
			codes = append(codes, code)
		}
	}
	return codes
}

func dateOnlyUTC(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
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
