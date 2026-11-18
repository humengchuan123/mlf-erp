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
	return scanCreditNote(row)
}

const creditNoteSelectSQL = `
	SELECT c.id::text, c.tenant_id::text, c.org_id::text, c.code, c.credit_type,
	       c.source_type, c.source_id::text, c.source_code, c.invoice_id::text,
	       COALESCE(i.code, ''),
	       c.partner_id::text, COALESCE(p.name, ''), c.note_date, c.currency,
	       c.exchange_rate::text, c.amount::text, c.base_amount::text,
	       c.applied_amount::text, c.applied_base_amount::text,
	       c.remaining_amount::text, c.remaining_base_amount::text,
	       c.status, c.reason, c.remark, c.created_by::text, c.created_at, c.updated_at
	FROM erp.finance_credit_notes c
	LEFT JOIN erp.partners p ON p.tenant_id = c.tenant_id AND p.id = c.partner_id
	LEFT JOIN erp.finance_invoices i ON i.tenant_id = c.tenant_id AND i.id = c.invoice_id`

func scanCreditNote(row rowScanner) (*domain.CreditNote, error) {
	var item domain.CreditNote
	var exchangeRate, amount, baseAmount, appliedAmount, appliedBaseAmount string
	var remainingAmount, remainingBaseAmount string

	err := row.Scan(
		&item.ID, &item.TenantID, &item.OrgID, &item.Code, &item.CreditType,
		&item.SourceType, &item.SourceID, &item.SourceCode, &item.InvoiceID,
		&item.InvoiceCode,
		&item.PartnerID, &item.PartnerName, &item.NoteDate, &item.Currency,
		&exchangeRate, &amount, &baseAmount,
		&appliedAmount, &appliedBaseAmount,
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
	item.RemainingAmount = decimal.RequireFromString(remainingAmount)
	item.RemainingBaseAmount = decimal.RequireFromString(remainingBaseAmount)
	return &item, nil
}
