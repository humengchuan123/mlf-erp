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

func (r *PeriodRepository) Close(ctx context.Context, tenantID, orgID, id, userID, voucherCode, fxVoucherCode, fxReversalCode string) error {
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
	yearClosed, err := yearClosedTx(ctx, tx, tenantID, period.Year)
	if err != nil {
		return err
	}
	if yearClosed {
		return domain.ErrYearClosed
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

	if err := revalueForeignInvoicesTx(ctx, tx, tenantID, orgID, period, userID, fxVoucherCode, fxReversalCode); err != nil {
		return err
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

func (r *PeriodRepository) Reopen(ctx context.Context, tenantID, orgID, id, userID, reversalCode, fxReopenCode, fxReversalReopenCode string) error {
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
	yearClosed, err := yearClosedTx(ctx, tx, tenantID, period.Year)
	if err != nil {
		return err
	}
	if yearClosed {
		return domain.ErrYearClosed
	}
	if period.Status == domain.PeriodStatusOpen {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		UPDATE erp.gl_periods
		SET status = 'open', closed_at = NULL, closed_by = NULL
		WHERE tenant_id = $1 AND id = $2`, tenantID, id); err != nil {
		return fmt.Errorf("open period for FX reversal: %w", err)
	}

	if err := undoFXRevaluationTx(ctx, tx, tenantID, orgID, userID, period, fxReopenCode, fxReversalReopenCode); err != nil {
		return err
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

	if closeID != "" {
		reversalVoucherID := ""
		if voucherID != "" {
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
				ID:                  uuid.NewString(),
				TenantID:            tenantID,
				OrgID:               orgID,
				Code:                reversalCode,
				VoucherDate:         period.EndDate,
				PeriodID:            period.ID,
				VoucherType:         domain.VoucherTypePeriodCloseReversal,
				SourceType:          domain.SourceTypePeriodCloseReversal,
				SourceID:            closeID,
				SourceCode:          fmt.Sprintf("%04d-%02d", period.Year, period.Month),
				Status:              domain.VoucherStatusPosted,
				Memo:                fmt.Sprintf("反结转 %04d-%02d", period.Year, period.Month),
				CreatedBy:           userID,
				PostedBy:            userID,
				ReversalOfVoucherID: voucherID,
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
			reversalVoucherID = reversal.ID
		}
		if _, err := tx.Exec(ctx, `
			UPDATE erp.gl_period_closes
			SET status = 'reopened', reversal_voucher_id = $3, reopened_by = $4, reopened_at = now()
			WHERE tenant_id = $1 AND id = $2`, tenantID, closeID, nullableUUID(reversalVoucherID), userID); err != nil {
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

	// Lock every period in the year so a concurrent reopen/close cannot change
	// the year between validation and posting.
	rows, err := tx.Query(ctx, `
		SELECT id::text FROM erp.gl_periods
		WHERE tenant_id = $1 AND year = $2
		ORDER BY month
		FOR UPDATE`, tenantID, year)
	if err != nil {
		return fmt.Errorf("lock year periods: %w", err)
	}
	periodCount := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan year period: %w", err)
		}
		periodCount++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if periodCount < 12 {
		return domain.ErrYearNotClosed
	}

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

	var laterClosed bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM erp.gl_year_closes WHERE tenant_id = $1 AND year > $2
		)`, tenantID, year).Scan(&laterClosed); err != nil {
		return fmt.Errorf("check later year close: %w", err)
	}
	if laterClosed {
		return domain.ErrYearOutOfOrder
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

	var recordedCount int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM erp.gl_periods p
		WHERE p.tenant_id = $1 AND p.year = $2 AND p.status = 'closed'
		  AND EXISTS (
		      SELECT 1 FROM erp.gl_period_closes c
		      WHERE c.tenant_id = p.tenant_id AND c.period_id = p.id AND c.status = 'closed'
		  )`, tenantID, year).Scan(&recordedCount); err != nil {
		return fmt.Errorf("count recorded period closes: %w", err)
	}
	if recordedCount < closedCount {
		return domain.ErrPeriodCloseMissing
	}

	var priorPeriods int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM erp.gl_periods WHERE tenant_id = $1 AND year = $2`,
		tenantID, year-1).Scan(&priorPeriods); err != nil {
		return fmt.Errorf("count prior year periods: %w", err)
	}
	if priorPeriods > 0 {
		var priorClosed bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM erp.gl_year_closes WHERE tenant_id = $1 AND year = $2
			)`, tenantID, year-1).Scan(&priorClosed); err != nil {
			return fmt.Errorf("check prior year close: %w", err)
		}
		if !priorClosed {
			return domain.ErrYearOutOfOrder
		}
	}

	profitAccounts, err := loadMappedAccountsTx(ctx, tx, tenantID, []postingLine{
		{AccountKey: "profit"},
	})
	if err != nil {
		return err
	}
	profitAccountID := profitAccounts["profit"]

	asOf := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
	var balanceRaw string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(l.credit - l.debit), 0)::text
		FROM erp.gl_voucher_lines l
		JOIN erp.gl_vouchers v ON v.tenant_id = l.tenant_id AND v.id = l.voucher_id
		WHERE l.tenant_id = $1 AND l.account_id = $2
		  AND v.voucher_date <= $3::date
		  AND v.status IN ('posted', 'reversed')`,
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
		retainedAccounts, err := loadMappedAccountsTx(ctx, tx, tenantID, []postingLine{
			{AccountKey: "retained_earnings"},
		})
		if err != nil {
			return err
		}
		retainedAccountID := retainedAccounts["retained_earnings"]

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
			VoucherType: domain.VoucherTypeYearClose,
			SourceType:  domain.SourceTypeYearClose,
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

func (r *PeriodRepository) YearCloseStatus(ctx context.Context, tenantID string, year int) (*domain.YearClose, error) {
	item := &domain.YearClose{}
	err := r.db.QueryRow(ctx, `
		SELECT c.id::text, c.tenant_id::text, c.year, COALESCE(c.voucher_id::text, ''),
		       COALESCE(v.code, ''), c.status, c.created_by::text, c.created_at
		FROM erp.gl_year_closes c
		LEFT JOIN erp.gl_vouchers v ON v.tenant_id = c.tenant_id AND v.id = c.voucher_id
		WHERE c.tenant_id = $1 AND c.year = $2`, tenantID, year,
	).Scan(&item.ID, &item.TenantID, &item.Year, &item.VoucherID,
		&item.VoucherCode, &item.Status, &item.CreatedBy, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load year close status: %w", err)
	}
	return item, nil
}

func yearClosedTx(ctx context.Context, tx pgx.Tx, tenantID string, year int) (bool, error) {
	var closed bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM erp.gl_year_closes WHERE tenant_id = $1 AND year = $2
		)`, tenantID, year).Scan(&closed); err != nil {
		return false, fmt.Errorf("check year close: %w", err)
	}
	return closed, nil
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
		closeID := uuid.NewString()
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.gl_period_closes (id, tenant_id, period_id, voucher_id, status, created_by)
			VALUES ($1, $2, $3, NULL, 'closed', $4)`,
			closeID, tenantID, period.ID, userID,
		); err != nil {
			return "", fmt.Errorf("insert empty period close: %w", err)
		}
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
		VoucherType: domain.VoucherTypePeriodClose,
		SourceType:  domain.SourceTypePeriodClose,
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

type fxRevaluationItem struct {
	InvoiceID      string
	InvoiceType    string
	PartnerID      string
	Currency       string
	ForeignBalance decimal.Decimal
	OldRate        decimal.Decimal
	NewRate        decimal.Decimal
	Adjustment     decimal.Decimal
}

func revalueForeignInvoicesTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, orgID string,
	period *domain.Period,
	userID, voucherCode, reversalCode string,
) error {
	rows, err := tx.Query(ctx, `
		SELECT i.id::text, i.invoice_type, i.partner_id::text, i.currency,
		       i.total_amount::text, i.paid_amount::text, i.credited_amount::text, i.exchange_rate::text,
		       i.base_total_amount::text, i.base_paid_amount::text, i.base_credited_amount::text
		FROM erp.finance_invoices i
		WHERE i.tenant_id = $1
		  AND i.currency <> 'CNY'
		  AND i.total_amount > i.paid_amount + i.credited_amount
		  AND i.invoice_date <= $2::date
		FOR UPDATE OF i`,
		tenantID, period.EndDate.Format("2006-01-02"),
	)
	if err != nil {
		return fmt.Errorf("list foreign invoices for revaluation: %w", err)
	}

	type invoiceRow struct {
		id, invoiceType, partnerID, currency             string
		foreign, rate, baseTotal, basePaid, baseCredited decimal.Decimal
	}
	invoices := make([]invoiceRow, 0)
	for rows.Next() {
		var item invoiceRow
		var totalRaw, paidRaw, creditedRaw, rateRaw, baseTotalRaw, basePaidRaw, baseCreditedRaw string
		if err := rows.Scan(
			&item.id, &item.invoiceType, &item.partnerID, &item.currency,
			&totalRaw, &paidRaw, &creditedRaw, &rateRaw, &baseTotalRaw, &basePaidRaw, &baseCreditedRaw,
		); err != nil {
			rows.Close()
			return fmt.Errorf("scan foreign invoice: %w", err)
		}
		total := decimal.RequireFromString(totalRaw)
		paid := decimal.RequireFromString(paidRaw)
		credited := decimal.RequireFromString(creditedRaw)
		item.foreign = total.Sub(paid).Sub(credited).Round(4)
		item.rate = decimal.RequireFromString(rateRaw)
		item.baseTotal = decimal.RequireFromString(baseTotalRaw)
		item.basePaid = decimal.RequireFromString(basePaidRaw)
		item.baseCredited = decimal.RequireFromString(baseCreditedRaw)
		invoices = append(invoices, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if len(invoices) == 0 {
		return nil
	}
	if voucherCode == "" || reversalCode == "" {
		return domain.ErrInvalidInput
	}

	rates := make(map[string]decimal.Decimal)
	items := make([]fxRevaluationItem, 0, len(invoices))
	zeroIDs := make([]string, 0)
	for _, invoice := range invoices {
		if invoice.foreign.IsZero() {
			continue
		}
		closingRate, ok := rates[invoice.currency]
		if !ok {
			if err := tx.QueryRow(ctx, `
				SELECT rate::text FROM erp.finance_exchange_rates
				WHERE tenant_id = $1 AND currency_code = $2 AND rate_date <= $3::date
				ORDER BY rate_date DESC LIMIT 1`,
				tenantID, invoice.currency, period.EndDate.Format("2006-01-02"),
			).Scan(&closingRate); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return domain.ErrFXRateNotFound
				}
				return fmt.Errorf("load closing rate: %w", err)
			}
			rates[invoice.currency] = closingRate
		}
		target := invoice.foreign.Mul(closingRate).Round(4)
		// The prior period's revaluation was already reversed in this period,
		// so the carrying amount before this close is the invoice-rate base
		// less base-currency payments and credits. base_revalued_amount is
		// replaced, not accumulated, with this period's adjustment.
		book := invoice.baseTotal.Sub(invoice.basePaid).Sub(invoice.baseCredited).Round(4)
		adjustment := target.Sub(book).Round(4)
		if adjustment.IsZero() {
			zeroIDs = append(zeroIDs, invoice.id)
			continue
		}
		items = append(items, fxRevaluationItem{
			InvoiceID: invoice.id, InvoiceType: invoice.invoiceType,
			PartnerID: invoice.partnerID, Currency: invoice.currency,
			ForeignBalance: invoice.foreign, OldRate: invoice.rate,
			NewRate: closingRate, Adjustment: adjustment,
		})
	}
	if len(items) == 0 {
		return resetInvoiceRevaluations(ctx, tx, tenantID, zeroIDs)
	}

	batchID := uuid.NewString()
	nextDate := period.EndDate.AddDate(0, 0, 1)
	nextPeriod, err := ensurePeriodTx(ctx, tx, tenantID, nextDate)
	if err != nil {
		return err
	}
	if nextPeriod.Status == domain.PeriodStatusClosed {
		return domain.ErrPeriodClosed
	}

	if err := postFXRevaluationVoucher(ctx, tx, tenantID, orgID, period, userID,
		voucherCode, domain.VoucherTypeFXRevaluation, domain.SourceTypeFXRevaluation,
		batchID, period.EndDate, fxRevaluationLines(items, false)); err != nil {
		return err
	}
	if err := postFXRevaluationVoucher(ctx, tx, tenantID, orgID, period, userID,
		reversalCode, domain.VoucherTypeFXRevaluationReversal, domain.SourceTypeFXRevaluationReversal,
		batchID, nextDate, fxRevaluationLines(items, true)); err != nil {
		return err
	}

	voucherID, err := findVoucherIDBySourceTx(ctx, tx, tenantID, domain.SourceTypeFXRevaluation, batchID)
	if err != nil {
		return err
	}
	reversalVoucherID, err := findVoucherIDBySourceTx(ctx, tx, tenantID, domain.SourceTypeFXRevaluationReversal, batchID)
	if err != nil {
		return err
	}

	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.gl_fx_revaluations (
				id, batch_id, tenant_id, period_id, invoice_id, invoice_type, partner_id,
				currency, foreign_balance, old_rate, new_rate, base_adjustment, status,
				voucher_id, reversal_voucher_id, created_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'posted',$13,$14,$15)`,
			uuid.NewString(), batchID, tenantID, period.ID, item.InvoiceID, item.InvoiceType,
			item.PartnerID, item.Currency, item.ForeignBalance.String(), item.OldRate.String(),
			item.NewRate.String(), item.Adjustment.String(), voucherID, reversalVoucherID, userID,
		); err != nil {
			return fmt.Errorf("insert FX revaluation: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE erp.finance_invoices
			SET base_revalued_amount = $3, updated_at = now()
			WHERE tenant_id = $1 AND id = $2`,
			tenantID, item.InvoiceID, item.Adjustment.String(),
		); err != nil {
			return fmt.Errorf("update invoice revaluation: %w", err)
		}
	}
	return resetInvoiceRevaluations(ctx, tx, tenantID, zeroIDs)
}

func resetInvoiceRevaluations(ctx context.Context, tx pgx.Tx, tenantID string, invoiceIDs []string) error {
	for _, invoiceID := range invoiceIDs {
		if _, err := tx.Exec(ctx, `
			UPDATE erp.finance_invoices
			SET base_revalued_amount = 0, updated_at = now()
			WHERE tenant_id = $1 AND id = $2`,
			tenantID, invoiceID,
		); err != nil {
			return fmt.Errorf("clear invoice revaluation: %w", err)
		}
	}
	return nil
}

func undoFXRevaluationTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, orgID, userID string,
	period *domain.Period,
	reopenCode, reversalReopenCode string,
) error {
	rows, err := tx.Query(ctx, `
		SELECT invoice_id::text, invoice_type, partner_id::text, base_adjustment::text
		FROM erp.gl_fx_revaluations
		WHERE tenant_id = $1 AND period_id = $2 AND status = 'posted'
		ORDER BY created_at
		FOR UPDATE`,
		tenantID, period.ID,
	)
	if err != nil {
		return fmt.Errorf("list FX revaluations for reopen: %w", err)
	}
	items := make([]fxRevaluationItem, 0)
	for rows.Next() {
		var item fxRevaluationItem
		var adjustmentRaw string
		if err := rows.Scan(&item.InvoiceID, &item.InvoiceType, &item.PartnerID, &adjustmentRaw); err != nil {
			rows.Close()
			return fmt.Errorf("scan FX revaluation for reopen: %w", err)
		}
		item.Adjustment = decimal.RequireFromString(adjustmentRaw)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(items) == 0 {
		return nil
	}
	if reopenCode == "" || reversalReopenCode == "" {
		return domain.ErrInvalidInput
	}

	nextDate := period.EndDate.AddDate(0, 0, 1)
	nextPeriod, err := ensurePeriodTx(ctx, tx, tenantID, nextDate)
	if err != nil {
		return err
	}
	if nextPeriod.Status == domain.PeriodStatusClosed {
		return domain.ErrPeriodClosed
	}
	reopenBatchID := uuid.NewString()
	if err := postFXRevaluationVoucher(ctx, tx, tenantID, orgID, period, userID,
		reopenCode, domain.VoucherTypeFXRevaluationReopen, domain.SourceTypeFXRevaluationReopen,
		reopenBatchID, period.EndDate, fxRevaluationLines(items, true)); err != nil {
		return err
	}
	if err := postFXRevaluationVoucher(ctx, tx, tenantID, orgID, period, userID,
		reversalReopenCode, domain.VoucherTypeFXRevaluationReopenReversal, domain.SourceTypeFXRevaluationReopenReversal,
		reopenBatchID, nextDate, fxRevaluationLines(items, false)); err != nil {
		return err
	}
	reopenVoucherID, err := findVoucherIDBySourceTx(ctx, tx, tenantID, domain.SourceTypeFXRevaluationReopen, reopenBatchID)
	if err != nil {
		return err
	}
	reversalReopenVoucherID, err := findVoucherIDBySourceTx(ctx, tx, tenantID, domain.SourceTypeFXRevaluationReopenReversal, reopenBatchID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			UPDATE erp.gl_fx_revaluations
			SET status = 'reopened', reopen_voucher_id = $3, reopen_reversal_voucher_id = $4,
			    reversed_at = now()
			WHERE tenant_id = $1 AND period_id = $2 AND invoice_id = $5 AND status = 'posted'`,
			tenantID, period.ID, reopenVoucherID, reversalReopenVoucherID, item.InvoiceID,
		); err != nil {
			return fmt.Errorf("mark FX revaluation reopened: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE erp.finance_invoices
			SET base_revalued_amount = base_revalued_amount - $3, updated_at = now()
			WHERE tenant_id = $1 AND id = $2`,
			tenantID, item.InvoiceID, item.Adjustment.String(),
		); err != nil {
			return fmt.Errorf("reverse invoice revaluation: %w", err)
		}
	}
	return nil
}

func postFXRevaluationVoucher(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, orgID string,
	period *domain.Period,
	userID, code, voucherType, sourceType, sourceID string,
	voucherDate time.Time,
	lines []postingLine,
) error {
	if len(lines) == 0 {
		return nil
	}
	return postVoucherTx(ctx, tx, voucherPostingInput{
		TenantID:    tenantID,
		OrgID:       orgID,
		VoucherCode: code,
		VoucherDate: voucherDate,
		VoucherType: voucherType,
		SourceType:  sourceType,
		SourceID:    sourceID,
		SourceCode:  fmt.Sprintf("%04d-%02d", period.Year, period.Month),
		Memo:        "外币重估 " + fmt.Sprintf("%04d-%02d", period.Year, period.Month),
		CreatedBy:   userID,
		Lines:       lines,
	})
}

func fxRevaluationLines(items []fxRevaluationItem, reverse bool) []postingLine {
	lines := make([]postingLine, 0, len(items)*2)
	for _, item := range items {
		adjustment := item.Adjustment.Round(4)
		if adjustment.IsZero() {
			continue
		}
		absolute := adjustment.Abs()
		var debitKey, creditKey, debitSummary, creditSummary string
		switch item.InvoiceType {
		case "receivable":
			if adjustment.IsPositive() {
				debitKey, debitSummary = "ar", "应收汇兑调整"
				creditKey, creditSummary = "fx_gain", "汇兑收益"
			} else {
				debitKey, debitSummary = "fx_loss", "汇兑损失"
				creditKey, creditSummary = "ar", "应收汇兑调整"
			}
		case "payable":
			if adjustment.IsPositive() {
				debitKey, debitSummary = "fx_loss", "汇兑损失"
				creditKey, creditSummary = "ap", "应付汇兑调整"
			} else {
				debitKey, debitSummary = "ap", "应付汇兑调整"
				creditKey, creditSummary = "fx_gain", "汇兑收益"
			}
		default:
			continue
		}
		lines = append(lines,
			postingLine{AccountKey: debitKey, Summary: debitSummary, Debit: absolute, PartnerID: item.PartnerID},
			postingLine{AccountKey: creditKey, Summary: creditSummary, Credit: absolute, PartnerID: item.PartnerID},
		)
	}
	if reverse {
		for i := range lines {
			lines[i].Debit, lines[i].Credit = lines[i].Credit, lines[i].Debit
			lines[i].Summary = "反重估 " + lines[i].Summary
		}
	}
	return lines
}

func findVoucherIDBySourceTx(ctx context.Context, tx pgx.Tx, tenantID, sourceType, sourceID string) (string, error) {
	var id string
	if err := tx.QueryRow(ctx, `
		SELECT id::text FROM erp.gl_vouchers
		WHERE tenant_id = $1 AND source_type = $2 AND source_id = $3`,
		tenantID, sourceType, sourceID).Scan(&id); err != nil {
		return "", fmt.Errorf("load FX voucher: %w", err)
	}
	return id, nil
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
