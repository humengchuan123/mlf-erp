package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/finance/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type InvoiceRepository struct {
	db *pgxpool.Pool
}

func NewInvoiceRepository(db *pgxpool.Pool) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

func (r *InvoiceRepository) List(
	ctx context.Context,
	filter scope.Filter,
	tenantID, invoiceType, status, vatInvoiceStatus, partnerID, keyword string,
	offset, limit int,
) ([]*domain.Invoice, int, error) {
	args := []any{tenantID}
	where := "i.tenant_id = $1"
	if invoiceType != "" {
		args = append(args, invoiceType)
		where += fmt.Sprintf(" AND i.invoice_type = $%d", len(args))
	}
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND i.status = $%d", len(args))
	}
	if vatInvoiceStatus != "" {
		args = append(args, vatInvoiceStatus)
		where += fmt.Sprintf(" AND i.vat_invoice_status = $%d", len(args))
	}
	if partnerID != "" {
		args = append(args, partnerID)
		where += fmt.Sprintf(" AND i.partner_id = $%d", len(args))
	}
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		index := len(args)
		where += fmt.Sprintf(" AND (i.code ILIKE $%d OR i.source_code ILIKE $%d OR i.vat_invoice_no ILIKE $%d OR p.name ILIKE $%d)", index, index, index, index)
	}
	appendScope(&where, &args, "i", filter)

	countQuery := `SELECT count(*) FROM erp.finance_invoices i
		LEFT JOIN erp.partners p ON p.tenant_id = i.tenant_id AND p.id = i.partner_id
		WHERE ` + where
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count invoices: %w", err)
	}

	query := invoiceSelectSQL + ` WHERE ` + where +
		fmt.Sprintf(` ORDER BY i.invoice_date DESC, i.created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list invoices: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Invoice, 0)
	for rows.Next() {
		item, scanErr := scanInvoice(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *InvoiceRepository) Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*domain.Invoice, error) {
	args := []any{tenantID, id}
	where := "i.tenant_id = $1 AND i.id = $2"
	appendScope(&where, &args, "i", filter)

	row := r.db.QueryRow(ctx, invoiceSelectSQL+` WHERE `+where, args...)
	return scanInvoice(row)
}

func (r *InvoiceRepository) Issue(
	ctx context.Context,
	filter scope.Filter,
	tenantID, id string,
	input domain.IssueInput,
) (*domain.Invoice, error) {
	invoice, err := r.Get(ctx, filter, tenantID, id)
	if err != nil {
		return nil, err
	}
	if invoice.VatInvoiceStatus == domain.VatInvoiceStatusVoid {
		return nil, domain.ErrInvoiceStatus
	}
	if invoice.VatInvoiceStatus == domain.VatInvoiceStatusIssued {
		return nil, domain.ErrInvoiceStatus
	}

	args := []any{tenantID, id, input.VatInvoiceNo, input.VatInvoiceType,
		input.VatInvoiceDate, input.BuyerTaxNo, input.SellerTaxNo}
	where := "i.tenant_id = $1 AND i.id = $2 AND i.vat_invoice_status = 'pending'"
	appendScope(&where, &args, "i", filter)

	tag, err := r.db.Exec(ctx, `
		UPDATE erp.finance_invoices AS i
		SET vat_invoice_no = $3, vat_invoice_type = $4, vat_invoice_date = $5,
		    buyer_tax_no = $6, seller_tax_no = $7, vat_invoice_status = 'issued',
		    updated_at = now()
		WHERE `+where, args...)
	if err != nil {
		if mapped := mapDBError(err); mapped != nil && mapped != err {
			return nil, domain.ErrVatInvoiceDuplicate
		}
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.ErrInvoiceStatus
	}
	return r.Get(ctx, filter, tenantID, id)
}

func (r *InvoiceRepository) Void(
	ctx context.Context,
	filter scope.Filter,
	tenantID, id, _ string,
) (*domain.Invoice, error) {
	invoice, err := r.Get(ctx, filter, tenantID, id)
	if err != nil {
		return nil, err
	}
	if invoice.VatInvoiceStatus != domain.VatInvoiceStatusIssued {
		return nil, domain.ErrInvoiceStatus
	}

	args := []any{tenantID, id}
	where := "i.tenant_id = $1 AND i.id = $2 AND i.vat_invoice_status = 'issued'"
	appendScope(&where, &args, "i", filter)
	tag, err := r.db.Exec(ctx, `
		UPDATE erp.finance_invoices AS i
		SET vat_invoice_status = 'void', updated_at = now()
		WHERE `+where, args...)
	if err != nil {
		return nil, mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.ErrInvoiceStatus
	}
	return r.Get(ctx, filter, tenantID, id)
}

func (r *InvoiceRepository) ListAllocations(ctx context.Context, filter scope.Filter, tenantID, invoiceID string) ([]*domain.Allocation, error) {
	args := []any{tenantID, invoiceID}
	where := "a.tenant_id = $1 AND a.invoice_id = $2"
	appendScope(&where, &args, "p", filter)

	rows, err := r.db.Query(ctx, `
		SELECT a.id::text, a.tenant_id::text, a.payment_id::text, a.invoice_id::text, a.amount::text,
		       a.created_by::text, a.created_at,
		       p.code, p.payment_type, p.payment_date
		FROM erp.finance_payment_allocations a
		JOIN erp.finance_payments p ON p.tenant_id = a.tenant_id AND p.id = a.payment_id
		WHERE `+where+`
		ORDER BY a.created_at`, args...)
	if err != nil {
		return nil, fmt.Errorf("list invoice allocations: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Allocation, 0)
	for rows.Next() {
		item := &domain.Allocation{}
		var amount string
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.PaymentID, &item.InvoiceID, &amount,
			&item.CreatedBy, &item.CreatedAt, &item.PaymentCode, &item.PaymentType, &item.PaymentDate,
		); err != nil {
			return nil, fmt.Errorf("scan invoice allocation: %w", err)
		}
		item.Amount = decimal.RequireFromString(amount)
		items = append(items, item)
	}
	return items, rows.Err()
}

const invoiceSelectSQL = `
	SELECT i.id::text, i.tenant_id::text, i.org_id::text, i.code, i.invoice_type,
	       i.source_type, i.source_id::text, i.source_code,
	       i.partner_id::text, COALESCE(p.name, ''), i.invoice_date, i.due_date,
	       i.currency, i.exchange_rate::text, i.status, i.net_amount::text, i.tax_amount::text,
	       i.total_amount::text, i.base_net_amount::text, i.base_tax_amount::text,
	       i.base_total_amount::text, i.base_paid_amount::text, i.base_credited_amount::text,
	       i.base_revalued_amount::text, i.paid_amount::text, i.credited_amount::text,
	       i.vat_invoice_no, i.vat_invoice_type, i.vat_invoice_date, i.vat_invoice_status,
	       i.buyer_tax_no, i.seller_tax_no,
	       i.remark, i.created_by::text, i.updated_by::text, i.created_at, i.updated_at
	FROM erp.finance_invoices i
	LEFT JOIN erp.partners p ON p.tenant_id = i.tenant_id AND p.id = i.partner_id`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanInvoice(row rowScanner) (*domain.Invoice, error) {
	var item domain.Invoice
	var exchangeRate, netAmount, taxAmount, total, baseNet, baseTax, baseTotal, basePaid, baseCredited, baseRevalued, paid, credited string
	var vatInvoiceDate sql.NullTime

	err := row.Scan(
		&item.ID, &item.TenantID, &item.OrgID, &item.Code, &item.InvoiceType,
		&item.SourceType, &item.SourceID, &item.SourceCode,
		&item.PartnerID, &item.PartnerName, &item.InvoiceDate, &item.DueDate,
		&item.Currency, &exchangeRate, &item.Status, &netAmount, &taxAmount,
		&total, &baseNet, &baseTax, &baseTotal, &basePaid, &baseCredited, &baseRevalued, &paid, &credited,
		&item.VatInvoiceNo, &item.VatInvoiceType, &vatInvoiceDate, &item.VatInvoiceStatus,
		&item.BuyerTaxNo, &item.SellerTaxNo,
		&item.Remark, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan invoice: %w", err)
	}
	item.ExchangeRate = decimal.RequireFromString(exchangeRate)
	item.NetAmount = decimal.RequireFromString(netAmount)
	item.TaxAmount = decimal.RequireFromString(taxAmount)
	item.TotalAmount = decimal.RequireFromString(total)
	item.BaseNetAmount = decimal.RequireFromString(baseNet)
	item.BaseTaxAmount = decimal.RequireFromString(baseTax)
	item.BaseTotalAmount = decimal.RequireFromString(baseTotal)
	item.BasePaidAmount = decimal.RequireFromString(basePaid)
	item.BaseCreditedAmount = decimal.RequireFromString(baseCredited)
	item.BaseRevaluedAmount = decimal.RequireFromString(baseRevalued)
	item.PaidAmount = decimal.RequireFromString(paid)
	item.CreditedAmount = decimal.RequireFromString(credited)
	if vatInvoiceDate.Valid {
		item.VatInvoiceDate = &vatInvoiceDate.Time
	}
	return &item, nil
}
