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
	"github.com/shopspring/decimal"
)

type PeriodRepository struct {
	db *pgxpool.Pool
}

func NewPeriodRepository(db *pgxpool.Pool) *PeriodRepository {
	return &PeriodRepository{db: db}
}

func (r *PeriodRepository) List(ctx context.Context, tenantID string, year int) ([]*domain.Period, error) {
	if year == 0 {
		year = time.Now().Year()
	}
	rows, err := r.db.Query(ctx, periodSelectSQL+`
		WHERE tenant_id = $1 AND year = $2 ORDER BY month`, tenantID, year)
	if err != nil {
		return nil, fmt.Errorf("list periods: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Period, 0)
	for rows.Next() {
		item, scanErr := scanPeriod(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PeriodRepository) Get(ctx context.Context, tenantID, id string) (*domain.Period, error) {
	row := r.db.QueryRow(ctx, periodSelectSQL+` WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return scanPeriod(row)
}

func (r *PeriodRepository) Ensure(ctx context.Context, tenantID string, date time.Time) (*domain.Period, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin ensure period: %w", err)
	}
	defer tx.Rollback(ctx)

	period, err := ensurePeriodTx(ctx, tx, tenantID, date)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit ensure period: %w", err)
	}
	return period, nil
}

func (r *PeriodRepository) EnsureYear(ctx context.Context, tenantID string, year int) error {
	if year == 0 {
		year = time.Now().Year()
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin ensure year: %w", err)
	}
	defer tx.Rollback(ctx)

	for month := 1; month <= 12; month++ {
		if _, err := ensurePeriodTx(ctx, tx, tenantID, time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit ensure year: %w", err)
	}
	return nil
}

func (r *PeriodRepository) Close(ctx context.Context, tenantID, orgID, id, userID, voucherCode string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin close period: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, periodSelectSQL+`
		WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, id)
	period, err := scanPeriod(row)
	if err != nil {
		return err
	}
	if period.Status == domain.PeriodStatusClosed {
		return domain.ErrConflict
	}

	var draftCount int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM erp.gl_vouchers
		WHERE tenant_id = $1 AND period_id = $2 AND status = 'draft'`, tenantID, id).Scan(&draftCount); err != nil {
		return fmt.Errorf("count draft vouchers: %w", err)
	}
	if draftCount > 0 {
		return domain.ErrConflict
	}

	if _, err := closePeriodTx(ctx, tx, tenantID, orgID, period, userID, voucherCode); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE erp.gl_periods
		SET status = 'closed', closed_at = now(), closed_by = $3
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, userID); err != nil {
		return fmt.Errorf("close period: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit close period: %w", err)
	}
	return nil
}

func (r *PeriodRepository) Reopen(ctx context.Context, tenantID, id, userID, reversalCode string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin reopen period: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, periodSelectSQL+`
		WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, id)
	period, err := scanPeriod(row)
	if err != nil {
		return err
	}
	if period.Status == domain.PeriodStatusOpen {
		return nil
	}

	var closeID, voucherID string
	err = tx.QueryRow(ctx, `
		SELECT id::text, COALESCE(voucher_id::text, '')
		FROM erp.gl_period_closes
		WHERE tenant_id = $1 AND period_id = $2 AND status = 'closed'
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE`, tenantID, id).Scan(&closeID, &voucherID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load period close: %w", err)
	}

	if closeID != "" && voucherID != "" {
		if reversalCode == "" {
			return domain.ErrInvalidInput
		}
		var orgID string
		if err := tx.QueryRow(ctx, `
			SELECT org_id::text FROM erp.gl_vouchers
			WHERE tenant_id = $1 AND id = $2`, tenantID, voucherID).Scan(&orgID); err != nil {
			return fmt.Errorf("load closing voucher: %w", err)
		}
		originalLines, err := loadVoucherLines(ctx, tx, tenantID, voucherID)
		if err != nil {
			return err
		}
		reversal := &domain.Voucher{
			ID:          uuid.NewString(),
			TenantID:    tenantID,
			OrgID:       orgID,
			Code:        reversalCode,
			VoucherDate: period.EndDate,
			PeriodID:    period.ID,
			VoucherType: "period_close_reversal",
			SourceType:  "period_close_reversal",
			SourceID:    closeID,
			SourceCode:  fmt.Sprintf("%04d-%02d", period.Year, period.Month),
			Status:      domain.VoucherStatusPosted,
			Memo:        fmt.Sprintf("反结转 %04d-%02d", period.Year, period.Month),
			CreatedBy:   userID,
			PostedBy:    userID,
		}
		now := time.Now()
		reversal.PostedAt = &now
		reversal.CreatedAt = now
		reversal.UpdatedAt = now
		total := decimal.Zero
		for i, line := range originalLines {
			total = total.Add(line.Credit)
			reversal.Lines = append(reversal.Lines, &domain.VoucherLine{
				ID:           uuid.NewString(),
				TenantID:     tenantID,
				VoucherID:    reversal.ID,
				LineNo:       i + 1,
				AccountID:    line.AccountID,
				AccountCode:  line.AccountCode,
				AccountName:  line.AccountName,
				Summary:      "反结转 " + line.Summary,
				Debit:        line.Credit,
				Credit:       line.Debit,
				PartnerID:    line.PartnerID,
				DepartmentID: line.DepartmentID,
				ProjectCode:  line.ProjectCode,
			})
		}
		reversal.TotalAmount = total.Round(4)
		if err := insertVoucherTx(ctx, tx, reversal); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE erp.gl_period_closes
			SET status = 'reopened', reversal_voucher_id = $3, reopened_by = $4, reopened_at = now()
			WHERE tenant_id = $1 AND id = $2`, tenantID, closeID, reversal.ID, userID); err != nil {
			return fmt.Errorf("mark period close reopened: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE erp.gl_periods
		SET status = 'open', closed_at = NULL, closed_by = NULL
		WHERE tenant_id = $1 AND id = $2`, tenantID, id); err != nil {
		return fmt.Errorf("reopen period: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reopen period: %w", err)
	}
	return nil
}

func (r *PeriodRepository) CloseYear(
	ctx context.Context,
	tenantID, orgID string,
	year int,
	userID, voucherCode string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin close year: %w", err)
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM erp.gl_year_closes WHERE tenant_id = $1 AND year = $2
		)`, tenantID, year).Scan(&exists); err != nil {
		return fmt.Errorf("check year close: %w", err)
	}
	if exists {
		return domain.ErrYearClosed
	}

	var closedCount int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM erp.gl_periods
		WHERE tenant_id = $1 AND year = $2 AND status = 'closed'`, tenantID, year).Scan(&closedCount); err != nil {
		return fmt.Errorf("count closed periods: %w", err)
	}
	if closedCount < 12 {
		return domain.ErrYearNotClosed
	}

	accounts, err := loadMappedAccountsTx(ctx, tx, tenantID, []postingLine{
		{AccountKey: "profit"},
		{AccountKey: "retained_earnings"},
	})
	if err != nil {
		return err
	}
	profitAccountID := accounts["profit"]
	retainedAccountID := accounts["retained_earnings"]

	asOf := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
	var balanceRaw string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(l.credit - l.debit), 0)::text
		FROM erp.gl_voucher_lines l
		JOIN erp.gl_vouchers v ON v.tenant_id = l.tenant_id AND v.id = l.voucher_id
		WHERE l.tenant_id = $1 AND l.account_id = $2
		  AND v.voucher_date <= $3::date
		  AND v.status IN ('posted', 'reversed')
		  AND v.voucher_type <> 'year_close'`,
		tenantID, profitAccountID, asOf.Format("2006-01-02"),
	).Scan(&balanceRaw); err != nil {
		return fmt.Errorf("load profit balance: %w", err)
	}
	balance := decimal.RequireFromString(balanceRaw)

	yearCloseID := uuid.NewString()
	var voucherID string
	if !balance.IsZero() {
		if voucherCode == "" {
			return domain.ErrInvalidInput
		}
		voucherDate := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)
		period, err := ensurePeriodTx(ctx, tx, tenantID, voucherDate)
		if err != nil {
			return err
		}
		if period.Status == domain.PeriodStatusClosed {
			return domain.ErrPeriodClosed
		}
		voucherID = uuid.NewString()
		voucher := &domain.Voucher{
			ID:          voucherID,
			TenantID:    tenantID,
			OrgID:       orgID,
			Code:        voucherCode,
			VoucherDate: voucherDate,
			PeriodID:    period.ID,
			VoucherType: "year_close",
			SourceType:  "year_close",
			SourceID:    yearCloseID,
			SourceCode:  fmt.Sprintf("%04d", year),
			Status:      domain.VoucherStatusPosted,
			Memo:        fmt.Sprintf("年度结转 %04d", year),
			CreatedBy:   userID,
			PostedBy:    userID,
		}
		now := time.Now()
		voucher.PostedAt = &now
		voucher.CreatedAt = now
		voucher.UpdatedAt = now
		amount := balance.Abs().Round(4)
		if balance.IsPositive() {
			voucher.Lines = []*domain.VoucherLine{
				{ID: uuid.NewString(), TenantID: tenantID, VoucherID: voucherID, LineNo: 1, AccountID: profitAccountID, Summary: "结转本年利润", Debit: amount},
				{ID: uuid.NewString(), TenantID: tenantID, VoucherID: voucherID, LineNo: 2, AccountID: retainedAccountID, Summary: "结转利润分配", Credit: amount},
			}
		} else {
			voucher.Lines = []*domain.VoucherLine{
				{ID: uuid.NewString(), TenantID: tenantID, VoucherID: voucherID, LineNo: 1, AccountID: retainedAccountID, Summary: "结转未弥补亏损", Debit: amount},
				{ID: uuid.NewString(), TenantID: tenantID, VoucherID: voucherID, LineNo: 2, AccountID: profitAccountID, Summary: "结转本年利润", Credit: amount},
			}
		}
		voucher.TotalAmount = amount
		if err := insertVoucherTx(ctx, tx, voucher); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.gl_year_closes (id, tenant_id, year, voucher_id, status, created_by)
		VALUES ($1, $2, $3, $4, 'closed', $5)`,
		yearCloseID, tenantID, year, nullableUUID(voucherID), userID,
	); err != nil {
		return fmt.Errorf("insert year close: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit close year: %w", err)
	}
	return nil
}

func closePeriodTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, orgID string,
	period *domain.Period,
	userID, voucherCode string,
) (string, error) {
	rows, err := tx.Query(ctx, `
		SELECT a.id::text, a.code, a.name, a.type, COALESCE(SUM(l.debit - l.credit), 0)::text
		FROM erp.gl_voucher_lines l
		JOIN erp.gl_vouchers v ON v.tenant_id = l.tenant_id AND v.id = l.voucher_id
		JOIN erp.gl_accounts a ON a.tenant_id = l.tenant_id AND a.id = l.account_id
		WHERE l.tenant_id = $1 AND v.period_id = $2
		  AND v.status IN ('posted', 'reversed')
		  AND v.voucher_type NOT IN ('period_close', 'period_close_reversal', 'year_close')
		  AND a.type IN ('revenue', 'expense')
		  AND a.is_leaf = true
		GROUP BY a.id, a.code, a.name, a.type
		HAVING SUM(l.debit - l.credit) <> 0
		ORDER BY a.code`, tenantID, period.ID)
	if err != nil {
		return "", fmt.Errorf("load period profit and loss: %w", err)
	}
	defer rows.Close()

	lines := make([]*domain.VoucherLine, 0)
	profitDebit := decimal.Zero
	profitCredit := decimal.Zero
	lineNo := 0
	addLine := func(accountID, summary string, debit, credit decimal.Decimal) {
		lineNo++
		lines = append(lines, &domain.VoucherLine{
			ID:        uuid.NewString(),
			TenantID:  tenantID,
			LineNo:    lineNo,
			AccountID: accountID,
			Summary:   summary,
			Debit:     debit.Round(4),
			Credit:    credit.Round(4),
		})
	}

	for rows.Next() {
		var accountID, code, name, accountType, signedRaw string
		if err := rows.Scan(&accountID, &code, &name, &accountType, &signedRaw); err != nil {
			return "", fmt.Errorf("scan period profit and loss: %w", err)
		}
		signed := decimal.RequireFromString(signedRaw)
		switch accountType {
		case domain.AccountTypeRevenue:
			if signed.IsNegative() {
				amount := signed.Neg()
				addLine(accountID, "结转收入 "+code+" "+name, amount, decimal.Zero)
				profitCredit = profitCredit.Add(amount)
			} else if signed.IsPositive() {
				amount := signed
				addLine(accountID, "结转收入 "+code+" "+name, decimal.Zero, amount)
				profitDebit = profitDebit.Add(amount)
			}
		case domain.AccountTypeExpense:
			if signed.IsPositive() {
				amount := signed
				addLine(accountID, "结转费用 "+code+" "+name, decimal.Zero, amount)
				profitDebit = profitDebit.Add(amount)
			} else if signed.IsNegative() {
				amount := signed.Neg()
				addLine(accountID, "结转费用 "+code+" "+name, amount, decimal.Zero)
				profitCredit = profitCredit.Add(amount)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", nil
	}

	net := profitCredit.Sub(profitDebit)
	if !net.IsZero() {
		accounts, err := loadMappedAccountsTx(ctx, tx, tenantID, []postingLine{{AccountKey: "profit"}})
		if err != nil {
			return "", err
		}
		if net.IsPositive() {
			addLine(accounts["profit"], "结转本年利润", decimal.Zero, net)
		} else {
			addLine(accounts["profit"], "结转本年利润", net.Neg(), decimal.Zero)
		}
	}

	totalDebit := decimal.Zero
	totalCredit := decimal.Zero
	for _, line := range lines {
		totalDebit = totalDebit.Add(line.Debit)
		totalCredit = totalCredit.Add(line.Credit)
	}
	if !totalDebit.Equal(totalCredit) || !totalDebit.IsPositive() {
		return "", domain.ErrUnbalanced
	}
	if voucherCode == "" {
		return "", domain.ErrInvalidInput
	}

	closeID := uuid.NewString()
	voucherID := uuid.NewString()
	for _, line := range lines {
		line.VoucherID = voucherID
	}
	now := time.Now()
	voucher := &domain.Voucher{
		ID:          voucherID,
		TenantID:    tenantID,
		OrgID:       orgID,
		Code:        voucherCode,
		VoucherDate: period.EndDate,
		PeriodID:    period.ID,
		VoucherType: "period_close",
		SourceType:  "period_close",
		SourceID:    closeID,
		SourceCode:  fmt.Sprintf("%04d-%02d", period.Year, period.Month),
		Status:      domain.VoucherStatusPosted,
		Memo:        fmt.Sprintf("损益结转 %04d-%02d", period.Year, period.Month),
		CreatedBy:   userID,
		PostedBy:    userID,
		PostedAt:    &now,
		CreatedAt:   now,
		UpdatedAt:   now,
		TotalAmount: totalDebit.Round(4),
		Lines:       lines,
	}
	if err := insertVoucherTx(ctx, tx, voucher); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.gl_period_closes (id, tenant_id, period_id, voucher_id, status, created_by)
		VALUES ($1, $2, $3, $4, 'closed', $5)`,
		closeID, tenantID, period.ID, voucherID, userID,
	); err != nil {
		return "", fmt.Errorf("insert period close: %w", err)
	}
	return voucherID, nil
}

func ensurePeriodTx(ctx context.Context, tx pgx.Tx, tenantID string, date time.Time) (*domain.Period, error) {
	year, month := date.Year(), int(date.Month())
	start := time.Date(year, date.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.gl_periods (id, tenant_id, year, month, start_date, end_date, status)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, 'open')
		ON CONFLICT (tenant_id, year, month) DO NOTHING`,
		tenantID, year, month, start, end,
	); err != nil {
		return nil, fmt.Errorf("ensure period: %w", err)
	}

	row := tx.QueryRow(ctx, periodSelectSQL+`
		WHERE tenant_id = $1 AND year = $2 AND month = $3
		FOR SHARE`, tenantID, year, month)
	return scanPeriod(row)
}

const periodSelectSQL = `
	SELECT id::text, tenant_id::text, year, month, start_date, end_date,
	       status, closed_at, COALESCE(closed_by::text, ''), created_at
	FROM erp.gl_periods`

func scanPeriod(row rowScanner) (*domain.Period, error) {
	var item domain.Period
	var closedAt sql.NullTime
	err := row.Scan(
		&item.ID, &item.TenantID, &item.Year, &item.Month, &item.StartDate, &item.EndDate,
		&item.Status, &closedAt, &item.ClosedBy, &item.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan period: %w", err)
	}
	if closedAt.Valid {
		item.ClosedAt = &closedAt.Time
	}
	return &item, nil
}
