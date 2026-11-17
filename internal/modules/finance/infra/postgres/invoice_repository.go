package postgres

import (
	"context"
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
	tenantID, invoiceType, status, partnerID, keyword string,
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
	if partnerID != "" {
		args = append(args, partnerID)
		where += fmt.Sprintf(" AND i.partner_id = $%d", len(args))
	}
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		index := len(args)
		where += fmt.Sprintf(" AND (i.code ILIKE $%d OR i.source_code ILIKE $%d OR p.name ILIKE $%d)", index, index, index)
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

func (r *InvoiceRepository) ListAllocations(ctx context.Context, filter scope.Filter, tenantID, invoiceID string) ([]*domain.Allocation, error) {
	_ = filter
	rows, err := r.db.Query(ctx, `
		SELECT a.id::text, a.tenant_id::text, a.payment_id::text, a.invoice_id::text, a.amount::text,
		       a.created_by::text, a.created_at,
		       p.code, p.payment_type, p.payment_date
		FROM erp.finance_payment_allocations a
		JOIN erp.finance_payments p ON p.tenant_id = a.tenant_id AND p.id = a.payment_id
		WHERE a.tenant_id = $1 AND a.invoice_id = $2
		ORDER BY a.created_at`, tenantID, invoiceID)
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
	       i.currency, i.status, i.total_amount::text, i.paid_amount::text,
	       i.remark, i.created_by::text, i.updated_by::text, i.created_at, i.updated_at
	FROM erp.finance_invoices i
	LEFT JOIN erp.partners p ON p.tenant_id = i.tenant_id AND p.id = i.partner_id`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanInvoice(row rowScanner) (*domain.Invoice, error) {
	var item domain.Invoice
	var total, paid string

	err := row.Scan(
		&item.ID, &item.TenantID, &item.OrgID, &item.Code, &item.InvoiceType,
		&item.SourceType, &item.SourceID, &item.SourceCode,
		&item.PartnerID, &item.PartnerName, &item.InvoiceDate, &item.DueDate,
		&item.Currency, &item.Status, &total, &paid,
		&item.Remark, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan invoice: %w", err)
	}
	item.TotalAmount = decimal.RequireFromString(total)
	item.PaidAmount = decimal.RequireFromString(paid)
	return &item, nil
}
