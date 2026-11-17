package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type VoucherRepository struct {
	db *pgxpool.Pool
}

func NewVoucherRepository(db *pgxpool.Pool) *VoucherRepository {
	return &VoucherRepository{db: db}
}

func (r *VoucherRepository) Create(ctx context.Context, filter scope.Filter, voucher *domain.Voucher, post bool) error {
	_ = filter
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create voucher: %w", err)
	}
	defer tx.Rollback(ctx)

	period, err := ensurePeriodTx(ctx, tx, voucher.TenantID, voucher.VoucherDate)
	if err != nil {
		return err
	}
	if period.Status == domain.PeriodStatusClosed {
		return domain.ErrPeriodClosed
	}
	voucher.PeriodID = period.ID
	voucher.Status = domain.VoucherStatusDraft
	if post {
		voucher.Status = domain.VoucherStatusPosted
		now := time.Now()
		voucher.PostedAt = &now
	}

	lines, err := validateAndBuildLinesTx(ctx, tx, voucher.TenantID, voucher.ID, voucher.InputLines)
	if err != nil {
		return err
	}
	voucher.Lines = lines
	if err := insertVoucherTx(ctx, tx, voucher); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create voucher: %w", err)
	}
	return nil
}

func (r *VoucherRepository) Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*domain.Voucher, error) {
	args := []any{tenantID, id}
	where := "v.tenant_id = $1 AND v.id = $2"
	appendScope(&where, &args, "v", filter)

	row := r.db.QueryRow(ctx, voucherSelectSQL+` WHERE `+where, args...)
	voucher, err := scanVoucher(row)
	if err != nil {
		return nil, err
	}
	lines, err := loadVoucherLines(ctx, r.db, tenantID, id)
	if err != nil {
		return nil, err
	}
	voucher.Lines = lines
	return voucher, nil
}

func (r *VoucherRepository) List(
	ctx context.Context,
	filter scope.Filter,
	tenantID, status, voucherType, keyword string,
	startDate, endDate time.Time,
	offset, limit int,
) ([]*domain.Voucher, int, error) {
	args := []any{tenantID}
	where := "v.tenant_id = $1"
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND v.status = $%d", len(args))
	}
	if voucherType != "" {
		args = append(args, voucherType)
		where += fmt.Sprintf(" AND v.voucher_type = $%d", len(args))
	}
	if !startDate.IsZero() {
		args = append(args, startDate.Format("2006-01-02"))
		where += fmt.Sprintf(" AND v.voucher_date >= $%d::date", len(args))
	}
	if !endDate.IsZero() {
		args = append(args, endDate.Format("2006-01-02"))
		where += fmt.Sprintf(" AND v.voucher_date <= $%d::date", len(args))
	}
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		index := len(args)
		where += fmt.Sprintf(" AND (v.code ILIKE $%d OR v.source_code ILIKE $%d OR v.memo ILIKE $%d)", index, index, index)
	}
	appendScope(&where, &args, "v", filter)

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM erp.gl_vouchers v WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count vouchers: %w", err)
	}

	query := voucherSelectSQL + ` WHERE ` + where +
		fmt.Sprintf(` ORDER BY v.voucher_date DESC, v.code DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list vouchers: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Voucher, 0)
	for rows.Next() {
		item, scanErr := scanVoucher(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *VoucherRepository) UpdateDraft(ctx context.Context, filter scope.Filter, voucher *domain.Voucher) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update voucher: %w", err)
	}
	defer tx.Rollback(ctx)

	args := []any{voucher.TenantID, voucher.ID}
	where := "v.tenant_id = $1 AND v.id = $2 AND v.status = 'draft'"
	appendScope(&where, &args, "v", filter)

	var status string
	err = tx.QueryRow(ctx, `SELECT v.status FROM erp.gl_vouchers v WHERE `+where+` FOR UPDATE OF v`, args...).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock draft voucher: %w", err)
	}

	period, err := ensurePeriodTx(ctx, tx, voucher.TenantID, voucher.VoucherDate)
	if err != nil {
		return err
	}
	if period.Status == domain.PeriodStatusClosed {
		return domain.ErrPeriodClosed
	}
	voucher.PeriodID = period.ID

	lines, err := validateAndBuildLinesTx(ctx, tx, voucher.TenantID, voucher.ID, voucher.InputLines)
	if err != nil {
		return err
	}
	voucher.Lines = lines

	if _, err := tx.Exec(ctx, `
		UPDATE erp.gl_vouchers
		SET voucher_date = $3, period_id = $4, memo = $5, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		voucher.TenantID, voucher.ID, voucher.VoucherDate, voucher.PeriodID, voucher.Memo,
	); err != nil {
		return fmt.Errorf("update voucher: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM erp.gl_voucher_lines WHERE tenant_id = $1 AND voucher_id = $2`,
		voucher.TenantID, voucher.ID); err != nil {
		return fmt.Errorf("delete voucher lines: %w", err)
	}
	if err := insertVoucherLinesTx(ctx, tx, voucher.Lines); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit update voucher: %w", err)
	}
	return nil
}

func (r *VoucherRepository) Post(ctx context.Context, filter scope.Filter, tenantID, id, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin post voucher: %w", err)
	}
	defer tx.Rollback(ctx)

	args := []any{tenantID, id}
	where := "v.tenant_id = $1 AND v.id = $2"
	appendScope(&where, &args, "v", filter)

	var status, periodID string
	err = tx.QueryRow(ctx, `
		SELECT v.status, v.period_id::text
		FROM erp.gl_vouchers v
		WHERE `+where+`
		FOR UPDATE OF v`, args...).Scan(&status, &periodID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock voucher: %w", err)
	}
	if status != domain.VoucherStatusDraft {
		return domain.ErrInvalidStatus
	}

	var periodStatus string
	if err := tx.QueryRow(ctx, `
		SELECT status FROM erp.gl_periods
		WHERE tenant_id = $1 AND id = $2`, tenantID, periodID).Scan(&periodStatus); err != nil {
		return fmt.Errorf("load voucher period: %w", err)
	}
	if periodStatus == domain.PeriodStatusClosed {
		return domain.ErrPeriodClosed
	}

	if err := validateVoucherBalanceTx(ctx, tx, tenantID, id); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE erp.gl_vouchers
		SET status = 'posted', posted_by = $3, posted_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, userID); err != nil {
		return fmt.Errorf("post voucher: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit post voucher: %w", err)
	}
	return nil
}

func (r *VoucherRepository) Reverse(
	ctx context.Context,
	filter scope.Filter,
	tenantID, id, reversalCode, userID string,
) (*domain.Voucher, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin reverse voucher: %w", err)
	}
	defer tx.Rollback(ctx)

	args := []any{tenantID, id}
	where := "v.tenant_id = $1 AND v.id = $2"
	appendScope(&where, &args, "v", filter)

	row := tx.QueryRow(ctx, voucherSelectSQL+` WHERE `+where+` FOR UPDATE OF v`, args...)
	original, err := scanVoucher(row)
	if err != nil {
		return nil, err
	}
	if original.Status != domain.VoucherStatusPosted {
		return nil, domain.ErrInvalidStatus
	}
	originalLines, err := loadVoucherLines(ctx, tx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if len(originalLines) == 0 {
		return nil, domain.ErrInvalidInput
	}

	reversalDate := time.Now()
	period, err := ensurePeriodTx(ctx, tx, tenantID, reversalDate)
	if err != nil {
		return nil, err
	}
	if period.Status == domain.PeriodStatusClosed {
		return nil, domain.ErrPeriodClosed
	}

	reversal := &domain.Voucher{
		ID:                  uuid.NewString(),
		TenantID:            tenantID,
		OrgID:               original.OrgID,
		Code:                reversalCode,
		VoucherDate:         reversalDate,
		PeriodID:            period.ID,
		VoucherType:         domain.VoucherTypeReversal,
		SourceType:          domain.SourceTypeReversal,
		SourceID:            original.ID,
		SourceCode:          original.Code,
		Status:              domain.VoucherStatusPosted,
		Memo:                "反过账 " + original.Code,
		CreatedBy:           userID,
		PostedBy:            userID,
		ReversalOfVoucherID: original.ID,
	}
	now := time.Now()
	reversal.PostedAt = &now

	for i, line := range originalLines {
		reversal.Lines = append(reversal.Lines, &domain.VoucherLine{
			ID:           uuid.NewString(),
			TenantID:     tenantID,
			VoucherID:    reversal.ID,
			LineNo:       i + 1,
			AccountID:    line.AccountID,
			AccountCode:  line.AccountCode,
			AccountName:  line.AccountName,
			Summary:      "反过账 " + line.Summary,
			Debit:        line.Credit,
			Credit:       line.Debit,
			PartnerID:    line.PartnerID,
			DepartmentID: line.DepartmentID,
		})
	}

	if err := insertVoucherTx(ctx, tx, reversal); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE erp.gl_vouchers
		SET status = 'reversed', updated_at = now()
		WHERE tenant_id = $1 AND id = $2`, tenantID, id); err != nil {
		return nil, fmt.Errorf("mark original voucher reversed: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit reverse voucher: %w", err)
	}
	return reversal, nil
}

const voucherSelectSQL = `
	SELECT v.id::text, v.tenant_id::text, v.org_id::text, v.code, v.voucher_date,
	       v.period_id::text, v.voucher_type, v.source_type, COALESCE(v.source_id::text, ''),
	       v.source_code, v.status, v.memo, v.created_by::text, COALESCE(v.posted_by::text, ''),
	       v.posted_at, COALESCE(v.reversal_of_voucher_id::text, ''), v.created_at, v.updated_at,
	       COALESCE((
	           SELECT SUM(l.debit) FROM erp.gl_voucher_lines l
	           WHERE l.tenant_id = v.tenant_id AND l.voucher_id = v.id
	       ), 0)::text
	FROM erp.gl_vouchers v`

func scanVoucher(row rowScanner) (*domain.Voucher, error) {
	var item domain.Voucher
	var postedAt sql.NullTime
	var total string
	err := row.Scan(
		&item.ID, &item.TenantID, &item.OrgID, &item.Code, &item.VoucherDate,
		&item.PeriodID, &item.VoucherType, &item.SourceType, &item.SourceID,
		&item.SourceCode, &item.Status, &item.Memo, &item.CreatedBy, &item.PostedBy,
		&postedAt, &item.ReversalOfVoucherID, &item.CreatedAt, &item.UpdatedAt, &total,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan voucher: %w", err)
	}
	if postedAt.Valid {
		item.PostedAt = &postedAt.Time
	}
	item.TotalAmount = decimal.RequireFromString(total)
	return &item, nil
}

func loadVoucherLines(ctx context.Context, q querier, tenantID, voucherID string) ([]*domain.VoucherLine, error) {
	rows, err := q.Query(ctx, `
		SELECT l.id::text, l.tenant_id::text, l.voucher_id::text, l.line_no,
		       l.account_id::text, a.code, a.name, l.summary, l.debit::text, l.credit::text,
		       COALESCE(l.partner_id::text, ''), COALESCE(l.department_id::text, '')
		FROM erp.gl_voucher_lines l
		JOIN erp.gl_accounts a ON a.tenant_id = l.tenant_id AND a.id = l.account_id
		WHERE l.tenant_id = $1 AND l.voucher_id = $2
		ORDER BY l.line_no`, tenantID, voucherID)
	if err != nil {
		return nil, fmt.Errorf("list voucher lines: %w", err)
	}
	defer rows.Close()

	lines := make([]*domain.VoucherLine, 0)
	for rows.Next() {
		line := &domain.VoucherLine{}
		var debit, credit string
		if err := rows.Scan(
			&line.ID, &line.TenantID, &line.VoucherID, &line.LineNo,
			&line.AccountID, &line.AccountCode, &line.AccountName, &line.Summary,
			&debit, &credit, &line.PartnerID, &line.DepartmentID,
		); err != nil {
			return nil, fmt.Errorf("scan voucher line: %w", err)
		}
		line.Debit = decimal.RequireFromString(debit)
		line.Credit = decimal.RequireFromString(credit)
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func validateAndBuildLinesTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, voucherID string,
	inputs []domain.VoucherLineInput,
) ([]*domain.VoucherLine, error) {
	if len(inputs) == 0 {
		return nil, domain.ErrInvalidInput
	}

	lines := make([]*domain.VoucherLine, 0, len(inputs))
	totalDebit := decimal.Zero
	totalCredit := decimal.Zero

	for i, input := range inputs {
		debit := input.Debit.Round(4)
		credit := input.Credit.Round(4)
		if debit.IsNegative() || credit.IsNegative() {
			return nil, domain.ErrInvalidInput
		}
		if !debit.IsPositive() && !credit.IsPositive() {
			return nil, domain.ErrInvalidInput
		}
		if debit.IsPositive() && credit.IsPositive() {
			return nil, domain.ErrInvalidInput
		}

		var isLeaf bool
		var status, accountCode, accountName string
		err := tx.QueryRow(ctx, `
			SELECT code, name, is_leaf, status FROM erp.gl_accounts
			WHERE tenant_id = $1 AND id = $2`, tenantID, input.AccountID).Scan(
			&accountCode, &accountName, &isLeaf, &status,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidInput
		}
		if err != nil {
			return nil, fmt.Errorf("load voucher account: %w", err)
		}
		if !isLeaf {
			return nil, domain.ErrAccountNotLeaf
		}
		if status != domain.StatusActive {
			return nil, domain.ErrAccountDisabled
		}

		totalDebit = totalDebit.Add(debit)
		totalCredit = totalCredit.Add(credit)
		lines = append(lines, &domain.VoucherLine{
			ID:           uuid.NewString(),
			TenantID:     tenantID,
			VoucherID:    voucherID,
			LineNo:       i + 1,
			AccountID:    input.AccountID,
			AccountCode:  accountCode,
			AccountName:  accountName,
			Summary:      input.Summary,
			Debit:        debit,
			Credit:       credit,
			PartnerID:    input.PartnerID,
			DepartmentID: input.DepartmentID,
		})
	}

	if !totalDebit.Equal(totalCredit) || !totalDebit.IsPositive() {
		return nil, domain.ErrUnbalanced
	}
	return lines, nil
}

func validateVoucherBalanceTx(ctx context.Context, tx pgx.Tx, tenantID, voucherID string) error {
	var debitRaw, creditRaw string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(debit), 0)::text, COALESCE(SUM(credit), 0)::text
		FROM erp.gl_voucher_lines
		WHERE tenant_id = $1 AND voucher_id = $2`, tenantID, voucherID).Scan(&debitRaw, &creditRaw); err != nil {
		return fmt.Errorf("sum voucher lines: %w", err)
	}
	debit := decimal.RequireFromString(debitRaw)
	credit := decimal.RequireFromString(creditRaw)
	if !debit.Equal(credit) || !debit.IsPositive() {
		return domain.ErrUnbalanced
	}
	return nil
}

func insertVoucherTx(ctx context.Context, tx pgx.Tx, voucher *domain.Voucher) error {
	postedBy := nullableUUID(voucher.PostedBy)
	if voucher.PostedAt != nil && voucher.PostedBy == "" {
		postedBy = nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.gl_vouchers (
			id, tenant_id, org_id, code, voucher_date, period_id, voucher_type,
			source_type, source_id, source_code, status, memo, created_by, posted_by,
			posted_at, reversal_of_voucher_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		voucher.ID, voucher.TenantID, voucher.OrgID, voucher.Code, voucher.VoucherDate,
		voucher.PeriodID, voucher.VoucherType, voucher.SourceType, nullableUUID(voucher.SourceID),
		voucher.SourceCode, voucher.Status, voucher.Memo, voucher.CreatedBy, postedBy,
		voucher.PostedAt, nullableUUID(voucher.ReversalOfVoucherID),
	); err != nil {
		return fmt.Errorf("insert voucher: %w", mapDBError(err))
	}
	return insertVoucherLinesTx(ctx, tx, voucher.Lines)
}

func insertVoucherLinesTx(ctx context.Context, tx pgx.Tx, lines []*domain.VoucherLine) error {
	for _, line := range lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.gl_voucher_lines (
				id, tenant_id, voucher_id, line_no, account_id, summary, debit, credit,
				partner_id, department_id
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			line.ID, line.TenantID, line.VoucherID, line.LineNo, line.AccountID, line.Summary,
			line.Debit.String(), line.Credit.String(),
			nullableUUID(line.PartnerID), nullableUUID(line.DepartmentID),
		); err != nil {
			return fmt.Errorf("insert voucher line: %w", mapDBError(err))
		}
	}
	return nil
}
