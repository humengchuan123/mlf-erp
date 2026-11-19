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

type CreditNoteRepository struct {
	db *pgxpool.Pool
}

func NewCreditNoteRepository(db *pgxpool.Pool) *CreditNoteRepository {
	return &CreditNoteRepository{db: db}
}

func (r *CreditNoteRepository) List(
	ctx context.Context,
	filter scope.Filter,
	tenantID, creditType, status, partnerID, keyword string,
	offset, limit int,
) ([]*domain.CreditNote, int, error) {
	args := []any{tenantID}
	where := "c.tenant_id = $1"
	if creditType != "" {
		args = append(args, creditType)
		where += fmt.Sprintf(" AND c.credit_type = $%d", len(args))
	}
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND c.status = $%d", len(args))
	}
	if partnerID != "" {
		args = append(args, partnerID)
		where += fmt.Sprintf(" AND c.partner_id = $%d", len(args))
	}
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		index := len(args)
		where += fmt.Sprintf(" AND (c.code ILIKE $%d OR c.source_code ILIKE $%d OR p.name ILIKE $%d OR p.code ILIKE $%d)", index, index, index, index)
	}
	appendScope(&where, &args, "c", filter)

	countQuery := `SELECT count(*) FROM erp.finance_credit_notes c
		LEFT JOIN erp.partners p ON p.tenant_id = c.tenant_id AND p.id = c.partner_id
		WHERE ` + where
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count credit notes: %w", err)
	}

	query := creditNoteSelectSQL + ` WHERE ` + where +
		fmt.Sprintf(` ORDER BY c.note_date DESC, c.code DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list credit notes: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.CreditNote, 0)
	for rows.Next() {
		item, scanErr := scanCreditNote(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *CreditNoteRepository) Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*domain.CreditNote, error) {
	args := []any{tenantID, id}
	where := "c.tenant_id = $1 AND c.id = $2"
	appendScope(&where, &args, "c", filter)

	row := r.db.QueryRow(ctx, creditNoteSelectSQL+` WHERE `+where, args...)
	item, err := scanCreditNote(row)
	if err != nil {
		return nil, err
	}
	if err := r.loadAllocations(ctx, item); err != nil {
		return nil, err
	}
	if err := r.loadRefunds(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (r *CreditNoteRepository) loadAllocations(ctx context.Context, item *domain.CreditNote) error {
	rows, err := r.db.Query(ctx, `
		SELECT a.id::text, a.tenant_id::text, a.org_id::text, a.credit_note_id::text,
		       a.invoice_id::text, i.code, i.invoice_date,
		       a.amount::text, a.base_amount::text, a.allocation_date,
		       a.created_by::text, a.created_at
		FROM erp.finance_credit_note_allocations a
		JOIN erp.finance_invoices i ON i.tenant_id = a.tenant_id AND i.id = a.invoice_id
		WHERE a.tenant_id = $1 AND a.credit_note_id = $2
		ORDER BY a.allocation_date, a.id`, item.TenantID, item.ID)
	if err != nil {
		return fmt.Errorf("list credit note allocations: %w", err)
	}
	defer rows.Close()

	item.Allocations = make([]*domain.CreditNoteAllocation, 0)
	for rows.Next() {
		allocation := &domain.CreditNoteAllocation{}
		var amount, baseAmount string
		if err := rows.Scan(
			&allocation.ID, &allocation.TenantID, &allocation.OrgID, &allocation.CreditNoteID,
			&allocation.InvoiceID, &allocation.InvoiceCode, &allocation.InvoiceDate,
			&amount, &baseAmount, &allocation.AllocationDate,
			&allocation.CreatedBy, &allocation.CreatedAt,
		); err != nil {
			return fmt.Errorf("scan credit note allocation: %w", err)
		}
		allocation.Amount = decimal.RequireFromString(amount)
		allocation.BaseAmount = decimal.RequireFromString(baseAmount)
		item.Allocations = append(item.Allocations, allocation)
	}
	return rows.Err()
}

func (r *CreditNoteRepository) loadRefunds(ctx context.Context, item *domain.CreditNote) error {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, tenant_id::text, org_id::text, code, credit_note_id::text,
		       refund_date, status, currency, exchange_rate::text, amount::text,
		       base_amount::text, cash_base_amount::text, fx_adjustment::text,
		       method, reference, remark, voucher_code, created_by::text, created_at
		FROM erp.finance_credit_note_refunds
		WHERE tenant_id = $1 AND credit_note_id = $2
		ORDER BY refund_date, id`, item.TenantID, item.ID)
	if err != nil {
		return fmt.Errorf("list credit note refunds: %w", err)
	}
	defer rows.Close()

	item.Refunds = make([]*domain.CreditNoteRefund, 0)
	for rows.Next() {
		refund := &domain.CreditNoteRefund{}
		var exchangeRate, amount, baseAmount, cashBaseAmount, fxAdjustment string
		if err := rows.Scan(
			&refund.ID, &refund.TenantID, &refund.OrgID, &refund.Code, &refund.CreditNoteID,
			&refund.RefundDate, &refund.Status, &refund.Currency, &exchangeRate,
			&amount, &baseAmount, &cashBaseAmount, &fxAdjustment,
			&refund.Method, &refund.Reference, &refund.Remark, &refund.VoucherCode,
			&refund.CreatedBy, &refund.CreatedAt,
		); err != nil {
			return fmt.Errorf("scan credit note refund: %w", err)
		}
		refund.ExchangeRate = decimal.RequireFromString(exchangeRate)
		refund.Amount = decimal.RequireFromString(amount)
		refund.BaseAmount = decimal.RequireFromString(baseAmount)
		refund.CashBaseAmount = decimal.RequireFromString(cashBaseAmount)
		refund.FXAdjustment = decimal.RequireFromString(fxAdjustment)
		item.Refunds = append(item.Refunds, refund)
	}
	return rows.Err()
}

func (r *CreditNoteRepository) Allocate(
	ctx context.Context,
	filter scope.Filter,
	tenantID, creditNoteID, userID string,
	allocationDate time.Time,
	allocations []*domain.AllocationInput,
) (*domain.CreditNote, error) {
	if len(allocations) == 0 {
		return nil, domain.ErrInvalidInput
	}

	// Canonicalize invoice IDs before duplicate detection so alternative UUID
	// spellings (upper case, braces, URN form) cannot bypass the check. The
	// canonical order still follows the request so base-currency calculation
	// is deterministic for the caller.
	canonical := make([]*domain.AllocationInput, 0, len(allocations))
	seen := make(map[string]struct{}, len(allocations))
	for _, allocation := range allocations {
		if allocation == nil || allocation.InvoiceID == "" {
			return nil, domain.ErrInvalidInput
		}
		amount := allocation.Amount.Round(4)
		if !amount.IsPositive() {
			return nil, domain.ErrInvalidInput
		}
		parsed, err := uuid.Parse(allocation.InvoiceID)
		if err != nil {
			return nil, domain.ErrInvalidInput
		}
		invoiceID := parsed.String()
		if _, exists := seen[invoiceID]; exists {
			return nil, domain.ErrInvalidInput
		}
		seen[invoiceID] = struct{}{}
		canonical = append(canonical, &domain.AllocationInput{
			InvoiceID: invoiceID,
			Amount:    amount,
		})
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin allocate credit note: %w", err)
	}
	defer tx.Rollback(ctx)

	args := []any{tenantID, creditNoteID}
	where := "c.tenant_id = $1 AND c.id = $2"
	appendScope(&where, &args, "c", filter)

	var note struct {
		creditType       string
		orgID            string
		partnerID        string
		currency         string
		status           string
		remainingRaw     string
		remainingBaseRaw string
		noteDate         time.Time
	}
	err = tx.QueryRow(ctx, `
		SELECT c.credit_type, c.org_id::text, c.partner_id::text, c.currency, c.status,
		       c.remaining_amount::text, c.remaining_base_amount::text, c.note_date
		FROM erp.finance_credit_notes c
		WHERE `+where+`
		FOR UPDATE OF c`, args...).Scan(
		&note.creditType, &note.orgID, &note.partnerID, &note.currency, &note.status,
		&note.remainingRaw, &note.remainingBaseRaw, &note.noteDate,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock credit note: %w", err)
	}
	if note.status != domain.CreditNoteStatusOpen {
		return nil, domain.ErrCreditNoteClosed
	}
	remainingAmount := decimal.RequireFromString(note.remainingRaw)
	remainingBase := decimal.RequireFromString(note.remainingBaseRaw)
	if !remainingAmount.IsPositive() || !remainingBase.IsPositive() {
		return nil, domain.ErrCreditNoteClosed
	}

	if allocationDate.IsZero() {
		allocationDate = time.Now()
	}
	allocationDate = time.Date(allocationDate.Year(), allocationDate.Month(), allocationDate.Day(), 0, 0, 0, 0, time.UTC)
	noteDate := time.Date(note.noteDate.Year(), note.noteDate.Month(), note.noteDate.Day(), 0, 0, 0, 0, time.UTC)
	if allocationDate.Before(noteDate) {
		return nil, domain.ErrInvalidInput
	}

	// Lock invoices in a stable order to avoid deadlocks between concurrent
	// allocations that touch the same invoice set. Locking is separate from
	// processing: allocations themselves are processed in request order.
	lockOrder := make([]string, 0, len(canonical))
	for _, allocation := range canonical {
		lockOrder = append(lockOrder, allocation.InvoiceID)
	}
	sort.Strings(lockOrder)

	type lockedInvoice struct {
		invoiceType     string
		partnerID       string
		orgID           string
		currency        string
		status          string
		totalRaw        string
		paidRaw         string
		creditedRaw     string
		baseTotalRaw    string
		basePaidRaw     string
		baseCreditedRaw string
	}
	invoices := make(map[string]*lockedInvoice, len(lockOrder))
	for _, invoiceID := range lockOrder {
		invoiceArgs := []any{tenantID, invoiceID}
		invoiceWhere := "i.tenant_id = $1 AND i.id = $2"
		appendScope(&invoiceWhere, &invoiceArgs, "i", filter)

		invoice := &lockedInvoice{}
		err = tx.QueryRow(ctx, `
			SELECT i.invoice_type, i.partner_id::text, i.org_id::text, i.currency, i.status,
			       i.total_amount::text, i.paid_amount::text, i.credited_amount::text,
			       i.base_total_amount::text, i.base_paid_amount::text, i.base_credited_amount::text
			FROM erp.finance_invoices i
			WHERE `+invoiceWhere+`
			FOR UPDATE OF i`, invoiceArgs...).Scan(
			&invoice.invoiceType, &invoice.partnerID, &invoice.orgID, &invoice.currency, &invoice.status,
			&invoice.totalRaw, &invoice.paidRaw, &invoice.creditedRaw,
			&invoice.baseTotalRaw, &invoice.basePaidRaw, &invoice.baseCreditedRaw,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("lock invoice for credit note allocation: %w", err)
		}
		invoices[invoiceID] = invoice
	}

	totalAmount := decimal.Zero
	totalBase := decimal.Zero
	for _, allocation := range canonical {
		invoice := invoices[allocation.InvoiceID]

		switch note.creditType {
		case domain.CreditNoteTypeSalesReturn:
			if invoice.invoiceType != domain.InvoiceTypeReceivable {
				return nil, domain.ErrInvoiceTypeMismatch
			}
		case domain.CreditNoteTypePurchaseReturn:
			if invoice.invoiceType != domain.InvoiceTypePayable {
				return nil, domain.ErrInvoiceTypeMismatch
			}
		default:
			return nil, domain.ErrInvalidInput
		}
		if invoice.partnerID != note.partnerID {
			return nil, domain.ErrPartnerMismatch
		}
		if invoice.currency != note.currency {
			return nil, domain.ErrCurrencyMismatch
		}
		if invoice.orgID != note.orgID {
			return nil, domain.ErrOrgMismatch
		}
		if invoice.status == domain.InvoiceStatusPaid {
			return nil, domain.ErrInvoicePaid
		}

		invoiceTotal := decimal.RequireFromString(invoice.totalRaw)
		invoicePaid := decimal.RequireFromString(invoice.paidRaw)
		invoiceCredited := decimal.RequireFromString(invoice.creditedRaw)
		outstanding := invoiceTotal.Sub(invoicePaid).Sub(invoiceCredited).Round(4)
		if !outstanding.IsPositive() {
			return nil, domain.ErrInvoicePaid
		}
		if allocation.Amount.GreaterThan(outstanding) {
			return nil, domain.ErrOverAllocate
		}
		if allocation.Amount.GreaterThan(remainingAmount) {
			return nil, domain.ErrOverApply
		}

		base := domain.AllocateBase(remainingAmount, remainingBase, allocation.Amount)
		invoiceBaseOutstanding := decimal.RequireFromString(invoice.baseTotalRaw).
			Sub(decimal.RequireFromString(invoice.basePaidRaw)).
			Sub(decimal.RequireFromString(invoice.baseCreditedRaw)).Round(4)
		if base.GreaterThan(invoiceBaseOutstanding) {
			return nil, domain.ErrInsufficientInvoiceBase
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.finance_credit_note_allocations (
				id, tenant_id, org_id, credit_note_id, invoice_id,
				amount, base_amount, allocation_date, created_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			uuid.NewString(), tenantID, note.orgID, creditNoteID, allocation.InvoiceID,
			allocation.Amount.String(), base.String(), allocationDate, userID,
		); err != nil {
			return nil, fmt.Errorf("insert credit note allocation: %w", mapDBError(err))
		}

		tag, err := tx.Exec(ctx, `
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
			  AND paid_amount + credited_amount + $3 <= total_amount
			  AND base_paid_amount + base_credited_amount + $4 <= base_total_amount`,
			tenantID, allocation.InvoiceID, allocation.Amount.String(), base.String(), userID)
		if err != nil {
			return nil, fmt.Errorf("update invoice credited amount: %w", err)
		}
		if tag.RowsAffected() != 1 {
			if base.GreaterThan(invoiceBaseOutstanding) {
				return nil, domain.ErrInsufficientInvoiceBase
			}
			return nil, domain.ErrOverAllocate
		}

		remainingAmount = remainingAmount.Sub(allocation.Amount)
		remainingBase = remainingBase.Sub(base)
		totalAmount = totalAmount.Add(allocation.Amount)
		totalBase = totalBase.Add(base)
	}

	status := domain.CreditNoteStatus(remainingAmount, remainingBase)
	tag, err := tx.Exec(ctx, `
		UPDATE erp.finance_credit_notes
		SET applied_amount = applied_amount + $3,
		    applied_base_amount = applied_base_amount + $4,
		    remaining_amount = remaining_amount - $3,
		    remaining_base_amount = remaining_base_amount - $4,
		    status = $5,
		    updated_at = now()
		WHERE tenant_id = $1 AND id = $2
		  AND status = 'open'
		  AND remaining_amount >= $3
		  AND remaining_base_amount >= $4`,
		tenantID, creditNoteID, totalAmount.String(), totalBase.String(), status)
	if err != nil {
		return nil, fmt.Errorf("update credit note applied amount: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return nil, domain.ErrOverApply
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit allocate credit note: %w", err)
	}
	return r.Get(ctx, filter, tenantID, creditNoteID)
}

func (r *CreditNoteRepository) Refund(
	ctx context.Context,
	filter scope.Filter,
	tenantID, creditNoteID, userID string,
	input domain.RefundInput,
	code, voucherCode string,
	exchangeRate decimal.Decimal,
) (*domain.CreditNote, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin refund credit note: %w", err)
	}
	defer tx.Rollback(ctx)

	args := []any{tenantID, creditNoteID}
	where := "c.tenant_id = $1 AND c.id = $2"
	appendScope(&where, &args, "c", filter)

	var note struct {
		creditType       string
		orgID            string
		partnerID        string
		currency         string
		status           string
		noteDate         time.Time
		remainingRaw     string
		remainingBaseRaw string
	}
	err = tx.QueryRow(ctx, `
		SELECT c.credit_type, c.org_id::text, c.partner_id::text, c.currency,
		       c.status, c.note_date, c.remaining_amount::text, c.remaining_base_amount::text
		FROM erp.finance_credit_notes c
		WHERE `+where+`
		FOR UPDATE OF c`, args...).Scan(
		&note.creditType, &note.orgID, &note.partnerID, &note.currency,
		&note.status, &note.noteDate, &note.remainingRaw, &note.remainingBaseRaw,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock credit note for refund: %w", err)
	}
	if note.status != domain.CreditNoteStatusOpen {
		return nil, domain.ErrCreditNoteClosed
	}
	remainingAmount := decimal.RequireFromString(note.remainingRaw)
	remainingBase := decimal.RequireFromString(note.remainingBaseRaw)
	if !remainingAmount.IsPositive() {
		return nil, domain.ErrCreditNoteClosed
	}

	refundDate := input.RefundDate
	if refundDate.IsZero() {
		refundDate = time.Now()
	}
	refundDate = time.Date(refundDate.Year(), refundDate.Month(), refundDate.Day(), 0, 0, 0, 0, time.UTC)
	noteDate := time.Date(note.noteDate.Year(), note.noteDate.Month(), note.noteDate.Day(), 0, 0, 0, 0, time.UTC)
	if refundDate.Before(noteDate) {
		return nil, domain.ErrInvalidInput
	}

	method := input.Method
	if method != "bank" && method != "cash" {
		return nil, domain.ErrInvalidInput
	}

	amount := input.Amount.Round(4)
	if !amount.IsPositive() || amount.GreaterThan(remainingAmount) {
		return nil, domain.ErrOverRefund
	}
	base := domain.AllocateBase(remainingAmount, remainingBase, amount)
	cashBase := domain.CashBase(amount, exchangeRate)
	if !cashBase.IsPositive() {
		return nil, domain.ErrRefundAmountTooSmall
	}
	fx := domain.FXAdjustment(cashBase, base)

	refundID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.finance_credit_note_refunds (
			id, tenant_id, org_id, code, credit_note_id, refund_date, status,
			currency, exchange_rate, amount, base_amount, cash_base_amount,
			fx_adjustment, method, reference, remark, voucher_code, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		refundID, tenantID, note.orgID, code, creditNoteID, refundDate,
		domain.CreditNoteRefundStatusPosted, note.currency, exchangeRate.String(),
		amount.String(), base.String(), cashBase.String(), fx.String(),
		method, input.Reference, input.Remark, voucherCode, userID,
	); err != nil {
		return nil, fmt.Errorf("insert credit note refund: %w", mapDBError(err))
	}

	status := domain.CreditNoteStatus(remainingAmount.Sub(amount), remainingBase.Sub(base))
	tag, err := tx.Exec(ctx, `
		UPDATE erp.finance_credit_notes
		SET refunded_amount = refunded_amount + $3,
		    refunded_base_amount = refunded_base_amount + $4,
		    remaining_amount = remaining_amount - $3,
		    remaining_base_amount = remaining_base_amount - $4,
		    status = $5,
		    updated_at = now()
		WHERE tenant_id = $1 AND id = $2
		  AND status = 'open'
		  AND remaining_amount >= $3
		  AND remaining_base_amount >= $4`,
		tenantID, creditNoteID, amount.String(), base.String(), status)
	if err != nil {
		return nil, fmt.Errorf("update credit note refund amounts: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return nil, domain.ErrOverRefund
	}

	if err := glpost.PostCreditNoteRefund(ctx, tx, glpost.CreditNoteRefundPosting{
		TenantID:       tenantID,
		OrgID:          note.orgID,
		VoucherCode:    voucherCode,
		SourceID:       refundID,
		SourceCode:     code,
		CreditType:     note.creditType,
		PartnerID:      note.partnerID,
		VoucherDate:    refundDate,
		BaseAmount:     base,
		CashBaseAmount: cashBase,
		FXAdjustment:   fx,
		Method:         method,
		CreatedBy:      userID,
	}); err != nil {
		return nil, fmt.Errorf("post credit note refund voucher: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit refund credit note: %w", err)
	}
	return r.Get(ctx, filter, tenantID, creditNoteID)
}

func (r *CreditNoteRepository) AllocationCandidates(
	ctx context.Context,
	filter scope.Filter,
	tenantID, creditNoteID string,
) ([]*domain.Invoice, error) {
	args := []any{tenantID, creditNoteID}
	where := "c.tenant_id = $1 AND c.id = $2"
	appendScope(&where, &args, "c", filter)

	var creditType, orgID, partnerID, currency, status, remainingAmountRaw, remainingBaseRaw string
	err := r.db.QueryRow(ctx, `
		SELECT c.credit_type, c.org_id::text, c.partner_id::text, c.currency,
		       c.status, c.remaining_amount::text, c.remaining_base_amount::text
		FROM erp.finance_credit_notes c
		WHERE `+where, args...).Scan(
		&creditType, &orgID, &partnerID, &currency, &status,
		&remainingAmountRaw, &remainingBaseRaw,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load credit note for allocation candidates: %w", err)
	}
	if status != domain.CreditNoteStatusOpen ||
		!decimal.RequireFromString(remainingAmountRaw).IsPositive() ||
		!decimal.RequireFromString(remainingBaseRaw).IsPositive() {
		return nil, domain.ErrCreditNoteClosed
	}

	invoiceType := ""
	switch creditType {
	case domain.CreditNoteTypeSalesReturn:
		invoiceType = domain.InvoiceTypeReceivable
	case domain.CreditNoteTypePurchaseReturn:
		invoiceType = domain.InvoiceTypePayable
	default:
		return nil, domain.ErrInvalidInput
	}

	args = []any{tenantID, orgID, partnerID, currency, invoiceType}
	where = `i.tenant_id = $1 AND i.org_id = $2 AND i.partner_id = $3
		AND i.currency = $4 AND i.invoice_type = $5
		AND i.status <> 'paid'
		AND i.paid_amount + i.credited_amount < i.total_amount
		AND i.base_paid_amount + i.base_credited_amount < i.base_total_amount`
	appendScope(&where, &args, "i", filter)

	rows, err := r.db.Query(ctx, invoiceSelectSQL+` WHERE `+where+`
		ORDER BY i.invoice_date, i.code`, args...)
	if err != nil {
		return nil, fmt.Errorf("list credit note allocation candidates: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Invoice, 0)
	for rows.Next() {
		item, scanErr := scanInvoice(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

const creditNoteSelectSQL = `
	SELECT c.id::text, c.tenant_id::text, c.org_id::text, c.code, c.credit_type,
	       c.source_type, c.source_id::text, c.source_code, c.invoice_id::text,
	       COALESCE(i.code, ''),
	       c.partner_id::text, COALESCE(p.name, ''), c.note_date, c.currency,
	       c.exchange_rate::text, c.amount::text, c.base_amount::text,
	       c.applied_amount::text, c.applied_base_amount::text,
	       c.refunded_amount::text, c.refunded_base_amount::text,
	       c.remaining_amount::text, c.remaining_base_amount::text,
	       c.status, c.reason, c.remark, c.created_by::text, c.created_at, c.updated_at
	FROM erp.finance_credit_notes c
	LEFT JOIN erp.partners p ON p.tenant_id = c.tenant_id AND p.id = c.partner_id
	LEFT JOIN erp.finance_invoices i ON i.tenant_id = c.tenant_id AND i.id = c.invoice_id`

func scanCreditNote(row rowScanner) (*domain.CreditNote, error) {
	var item domain.CreditNote
	var exchangeRate, amount, baseAmount, appliedAmount, appliedBaseAmount string
	var refundedAmount, refundedBaseAmount string
	var remainingAmount, remainingBaseAmount string

	err := row.Scan(
		&item.ID, &item.TenantID, &item.OrgID, &item.Code, &item.CreditType,
		&item.SourceType, &item.SourceID, &item.SourceCode, &item.InvoiceID,
		&item.InvoiceCode,
		&item.PartnerID, &item.PartnerName, &item.NoteDate, &item.Currency,
		&exchangeRate, &amount, &baseAmount,
		&appliedAmount, &appliedBaseAmount,
		&refundedAmount, &refundedBaseAmount,
		&remainingAmount, &remainingBaseAmount,
		&item.Status, &item.Reason, &item.Remark, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan credit note: %w", err)
	}
	item.ExchangeRate = decimal.RequireFromString(exchangeRate)
	item.Amount = decimal.RequireFromString(amount)
	item.BaseAmount = decimal.RequireFromString(baseAmount)
	item.AppliedAmount = decimal.RequireFromString(appliedAmount)
	item.AppliedBaseAmount = decimal.RequireFromString(appliedBaseAmount)
	item.RefundedAmount = decimal.RequireFromString(refundedAmount)
	item.RefundedBaseAmount = decimal.RequireFromString(refundedBaseAmount)
	item.RemainingAmount = decimal.RequireFromString(remainingAmount)
	item.RemainingBaseAmount = decimal.RequireFromString(remainingBaseAmount)
	return &item, nil
}
