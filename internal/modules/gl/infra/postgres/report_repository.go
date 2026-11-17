package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type ReportRepository struct {
	db *pgxpool.Pool
}

func NewReportRepository(db *pgxpool.Pool) *ReportRepository {
	return &ReportRepository{db: db}
}

func (r *ReportRepository) TrialBalance(
	ctx context.Context,
	filter scope.Filter,
	tenantID string,
	startDate, endDate time.Time,
) (*domain.TrialBalance, error) {
	accounts, err := r.listLeafAccounts(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	args := []any{tenantID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")}
	where := "l.tenant_id = $1 AND v.status IN ('posted', 'reversed')"
	appendScope(&where, &args, "v", filter)

	query := fmt.Sprintf(`
		SELECT l.account_id::text,
		       COALESCE(SUM(CASE WHEN v.voucher_date < $2::date THEN l.debit - l.credit ELSE 0 END), 0)::text,
		       COALESCE(SUM(CASE WHEN v.voucher_date BETWEEN $2::date AND $3::date THEN l.debit ELSE 0 END), 0)::text,
		       COALESCE(SUM(CASE WHEN v.voucher_date BETWEEN $2::date AND $3::date THEN l.credit ELSE 0 END), 0)::text
		FROM erp.gl_voucher_lines l
		JOIN erp.gl_vouchers v ON v.tenant_id = l.tenant_id AND v.id = l.voucher_id
		WHERE %s
		GROUP BY l.account_id`, where)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("trial balance aggregates: %w", err)
	}
	defer rows.Close()

	type aggregate struct {
		opening, periodDebit, periodCredit decimal.Decimal
	}
	byAccount := make(map[string]aggregate)
	for rows.Next() {
		var accountID, openingRaw, debitRaw, creditRaw string
		if err := rows.Scan(&accountID, &openingRaw, &debitRaw, &creditRaw); err != nil {
			return nil, fmt.Errorf("scan trial balance aggregate: %w", err)
		}
		byAccount[accountID] = aggregate{
			opening:      decimal.RequireFromString(openingRaw),
			periodDebit:  decimal.RequireFromString(debitRaw),
			periodCredit: decimal.RequireFromString(creditRaw),
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := &domain.TrialBalance{
		StartDate: startDate,
		EndDate:   endDate,
		Rows:      make([]*domain.TrialBalanceRow, 0, len(accounts)),
	}
	for _, account := range accounts {
		agg := byAccount[account.ID]
		closing := agg.opening.Add(agg.periodDebit).Sub(agg.periodCredit)
		row := &domain.TrialBalanceRow{
			AccountID:    account.ID,
			Code:         account.Code,
			Name:         account.Name,
			Type:         account.Type,
			Direction:    account.Direction,
			Opening:      agg.opening,
			PeriodDebit:  agg.periodDebit,
			PeriodCredit: agg.periodCredit,
			Closing:      closing,
		}
		result.Rows = append(result.Rows, row)
		result.TotalDebit = result.TotalDebit.Add(agg.periodDebit)
		result.TotalCredit = result.TotalCredit.Add(agg.periodCredit)
	}
	return result, nil
}

func (r *ReportRepository) GeneralLedger(
	ctx context.Context,
	filter scope.Filter,
	tenantID, accountID string,
	startDate, endDate time.Time,
) (*domain.GeneralLedger, error) {
	account, err := r.getLeafAccount(ctx, tenantID, accountID)
	if err != nil {
		return nil, err
	}

	openingArgs := []any{tenantID, accountID, startDate.Format("2006-01-02")}
	openingWhere := "l.tenant_id = $1 AND l.account_id = $2 AND v.status IN ('posted', 'reversed') AND v.voucher_date < $3::date"
	appendScope(&openingWhere, &openingArgs, "v", filter)

	var openingRaw string
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(l.debit - l.credit), 0)::text
		FROM erp.gl_voucher_lines l
		JOIN erp.gl_vouchers v ON v.tenant_id = l.tenant_id AND v.id = l.voucher_id
		WHERE `+openingWhere, openingArgs...).Scan(&openingRaw); err != nil {
		return nil, fmt.Errorf("general ledger opening: %w", err)
	}
	opening := decimal.RequireFromString(openingRaw)

	args := []any{tenantID, accountID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")}
	where := "l.tenant_id = $1 AND l.account_id = $2 AND v.status IN ('posted', 'reversed') AND v.voucher_date BETWEEN $3::date AND $4::date"
	appendScope(&where, &args, "v", filter)

	rows, err := r.db.Query(ctx, `
		SELECT v.id::text, v.code, v.voucher_date, l.summary, l.debit::text, l.credit::text
		FROM erp.gl_voucher_lines l
		JOIN erp.gl_vouchers v ON v.tenant_id = l.tenant_id AND v.id = l.voucher_id
		WHERE `+where+`
		ORDER BY v.voucher_date, v.code, l.line_no`, args...)
	if err != nil {
		return nil, fmt.Errorf("general ledger entries: %w", err)
	}
	defer rows.Close()

	entries := make([]*domain.GeneralLedgerEntry, 0)
	balance := opening
	for rows.Next() {
		entry := &domain.GeneralLedgerEntry{}
		var debitRaw, creditRaw string
		if err := rows.Scan(&entry.VoucherID, &entry.VoucherCode, &entry.VoucherDate, &entry.Summary, &debitRaw, &creditRaw); err != nil {
			return nil, fmt.Errorf("scan general ledger entry: %w", err)
		}
		entry.Debit = decimal.RequireFromString(debitRaw)
		entry.Credit = decimal.RequireFromString(creditRaw)
		balance = balance.Add(entry.Debit).Sub(entry.Credit)
		entry.Balance = balance
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &domain.GeneralLedger{
		Account:   account,
		StartDate: startDate,
		EndDate:   endDate,
		Opening:   opening,
		Entries:   entries,
		Closing:   balance,
	}, nil
}

func (r *ReportRepository) BalanceSheet(
	ctx context.Context,
	filter scope.Filter,
	tenantID string,
	asOf time.Time,
) (*domain.BalanceSheet, error) {
	args := []any{tenantID, asOf.Format("2006-01-02")}
	where := "l.tenant_id = $1 AND v.status IN ('posted', 'reversed') AND v.voucher_date <= $2::date"
	appendScope(&where, &args, "v", filter)

	rows, err := r.db.Query(ctx, `
		SELECT a.code, a.name, a.type, COALESCE(SUM(l.debit - l.credit), 0)::text
		FROM erp.gl_voucher_lines l
		JOIN erp.gl_vouchers v ON v.tenant_id = l.tenant_id AND v.id = l.voucher_id
		JOIN erp.gl_accounts a ON a.tenant_id = l.tenant_id AND a.id = l.account_id
		WHERE `+where+`
		GROUP BY a.code, a.name, a.type
		ORDER BY a.code`, args...)
	if err != nil {
		return nil, fmt.Errorf("balance sheet: %w", err)
	}
	defer rows.Close()

	result := &domain.BalanceSheet{
		AsOf:        asOf,
		Assets:      make([]*domain.ReportLine, 0),
		Liabilities: make([]*domain.ReportLine, 0),
		Equity:      make([]*domain.ReportLine, 0),
	}
	revenue := decimal.Zero
	expense := decimal.Zero

	for rows.Next() {
		var code, name, accountType, signedRaw string
		if err := rows.Scan(&code, &name, &accountType, &signedRaw); err != nil {
			return nil, fmt.Errorf("scan balance sheet row: %w", err)
		}
		signed := decimal.RequireFromString(signedRaw)
		line := &domain.ReportLine{Code: code, Name: name}
		switch accountType {
		case domain.AccountTypeAsset:
			line.Amount = signed
			if !line.Amount.IsZero() {
				result.Assets = append(result.Assets, line)
				result.TotalAssets = result.TotalAssets.Add(line.Amount)
			}
		case domain.AccountTypeLiability:
			line.Amount = signed.Neg()
			if !line.Amount.IsZero() {
				result.Liabilities = append(result.Liabilities, line)
				result.TotalLiabilities = result.TotalLiabilities.Add(line.Amount)
			}
		case domain.AccountTypeEquity:
			line.Amount = signed.Neg()
			if !line.Amount.IsZero() {
				result.Equity = append(result.Equity, line)
				result.TotalEquity = result.TotalEquity.Add(line.Amount)
			}
		case domain.AccountTypeRevenue:
			revenue = revenue.Sub(signed)
		case domain.AccountTypeExpense:
			expense = expense.Add(signed)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result.CurrentProfit = revenue.Sub(expense)
	result.TotalLiabilitiesAndEquity = result.TotalLiabilities.Add(result.TotalEquity).Add(result.CurrentProfit)
	return result, nil
}

func (r *ReportRepository) AuxiliaryBalance(
	ctx context.Context,
	filter scope.Filter,
	tenantID, dimension, accountID string,
	startDate, endDate time.Time,
) (*domain.AuxiliaryBalance, error) {
	var dimensionID, dimensionCode, dimensionName, dimensionJoin, dimensionFilter string
	switch dimension {
	case "partner":
		dimensionID = "COALESCE(l.partner_id::text, '')"
		dimensionCode = "COALESCE(p.code, '')"
		dimensionName = "COALESCE(p.name, '')"
		dimensionJoin = "LEFT JOIN erp.partners p ON p.tenant_id = l.tenant_id AND p.id = l.partner_id"
		dimensionFilter = "l.partner_id IS NOT NULL"
	case "department":
		dimensionID = "COALESCE(l.department_id::text, '')"
		dimensionCode = "COALESCE(o.code, '')"
		dimensionName = "COALESCE(o.name, '')"
		dimensionJoin = "LEFT JOIN erp.organizations o ON o.tenant_id = l.tenant_id AND o.id = l.department_id AND o.type = 'dept'"
		dimensionFilter = "l.department_id IS NOT NULL AND o.type = 'dept'"
	case "project":
		dimensionID = "l.project_code"
		dimensionCode = "l.project_code"
		dimensionName = "l.project_code"
		dimensionJoin = ""
		dimensionFilter = "l.project_code <> ''"
	default:
		return nil, domain.ErrInvalidInput
	}

	args := []any{tenantID}
	where := "l.tenant_id = $1 AND v.status IN ('posted', 'reversed') AND v.voucher_type NOT IN ('period_close', 'period_close_reversal', 'year_close')"
	if accountID != "" {
		args = append(args, accountID)
		where += fmt.Sprintf(" AND l.account_id = $%d", len(args))
	}
	args = append(args, startDate.Format("2006-01-02"))
	startIndex := len(args)
	args = append(args, endDate.Format("2006-01-02"))
	endIndex := len(args)
	where += fmt.Sprintf(" AND %s", dimensionFilter)
	where += fmt.Sprintf(" AND v.voucher_date <= $%d::date", endIndex)
	appendScope(&where, &args, "v", filter)

	query := fmt.Sprintf(`
		SELECT l.account_id::text, a.code, a.name,
		       %s, %s, %s,
		       COALESCE(SUM(CASE WHEN v.voucher_date < $%d::date THEN l.debit - l.credit ELSE 0 END), 0)::text,
		       COALESCE(SUM(CASE WHEN v.voucher_date BETWEEN $%d::date AND $%d::date THEN l.debit ELSE 0 END), 0)::text,
		       COALESCE(SUM(CASE WHEN v.voucher_date BETWEEN $%d::date AND $%d::date THEN l.credit ELSE 0 END), 0)::text
		FROM erp.gl_voucher_lines l
		JOIN erp.gl_vouchers v ON v.tenant_id = l.tenant_id AND v.id = l.voucher_id
		JOIN erp.gl_accounts a ON a.tenant_id = l.tenant_id AND a.id = l.account_id
		%s
		WHERE %s
		GROUP BY l.account_id, a.code, a.name, %s, %s, %s
		ORDER BY a.code, %s`,
		dimensionID, dimensionCode, dimensionName,
		startIndex, startIndex, endIndex, startIndex, endIndex,
		dimensionJoin, where, dimensionID, dimensionCode, dimensionName, dimensionID,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("auxiliary balance: %w", err)
	}
	defer rows.Close()

	result := &domain.AuxiliaryBalance{
		Dimension: dimension,
		StartDate: startDate,
		EndDate:   endDate,
		Rows:      make([]*domain.AuxiliaryBalanceRow, 0),
	}
	for rows.Next() {
		row := &domain.AuxiliaryBalanceRow{}
		var openingRaw, debitRaw, creditRaw string
		if err := rows.Scan(
			&row.AccountID, &row.AccountCode, &row.AccountName,
			&row.DimensionID, &row.DimensionCode, &row.DimensionName,
			&openingRaw, &debitRaw, &creditRaw,
		); err != nil {
			return nil, fmt.Errorf("scan auxiliary balance: %w", err)
		}
		row.Opening = decimal.RequireFromString(openingRaw)
		row.PeriodDebit = decimal.RequireFromString(debitRaw)
		row.PeriodCredit = decimal.RequireFromString(creditRaw)
		row.Closing = row.Opening.Add(row.PeriodDebit).Sub(row.PeriodCredit)
		result.Rows = append(result.Rows, row)
		result.TotalDebit = result.TotalDebit.Add(row.PeriodDebit)
		result.TotalCredit = result.TotalCredit.Add(row.PeriodCredit)
	}
	return result, rows.Err()
}

func (r *ReportRepository) IncomeStatement(
	ctx context.Context,
	filter scope.Filter,
	tenantID string,
	startDate, endDate time.Time,
) (*domain.IncomeStatement, error) {
	args := []any{tenantID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")}
	where := "l.tenant_id = $1 AND v.status IN ('posted', 'reversed') AND v.voucher_date BETWEEN $2::date AND $3::date AND v.voucher_type NOT IN ('period_close', 'period_close_reversal', 'year_close')"
	appendScope(&where, &args, "v", filter)

	rows, err := r.db.Query(ctx, `
		SELECT a.code, a.name, a.type, COALESCE(SUM(l.debit - l.credit), 0)::text
		FROM erp.gl_voucher_lines l
		JOIN erp.gl_vouchers v ON v.tenant_id = l.tenant_id AND v.id = l.voucher_id
		JOIN erp.gl_accounts a ON a.tenant_id = l.tenant_id AND a.id = l.account_id
		WHERE `+where+` AND a.type IN ('revenue', 'expense')
		GROUP BY a.code, a.name, a.type
		ORDER BY a.code`, args...)
	if err != nil {
		return nil, fmt.Errorf("income statement: %w", err)
	}
	defer rows.Close()

	result := &domain.IncomeStatement{
		StartDate: startDate,
		EndDate:   endDate,
		Revenue:   make([]*domain.ReportLine, 0),
		Expenses:  make([]*domain.ReportLine, 0),
	}
	for rows.Next() {
		var code, name, accountType, signedRaw string
		if err := rows.Scan(&code, &name, &accountType, &signedRaw); err != nil {
			return nil, fmt.Errorf("scan income statement row: %w", err)
		}
		signed := decimal.RequireFromString(signedRaw)
		line := &domain.ReportLine{Code: code, Name: name}
		if accountType == domain.AccountTypeRevenue {
			line.Amount = signed.Neg()
			if !line.Amount.IsZero() {
				result.Revenue = append(result.Revenue, line)
				result.TotalRevenue = result.TotalRevenue.Add(line.Amount)
			}
		} else {
			line.Amount = signed
			if !line.Amount.IsZero() {
				result.Expenses = append(result.Expenses, line)
				result.TotalExpenses = result.TotalExpenses.Add(line.Amount)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result.NetProfit = result.TotalRevenue.Sub(result.TotalExpenses)
	return result, nil
}

func (r *ReportRepository) listLeafAccounts(ctx context.Context, tenantID string) ([]*domain.Account, error) {
	rows, err := r.db.Query(ctx, accountSelectSQL+`
		WHERE tenant_id = $1
		  AND (
		      is_leaf = true
		      OR EXISTS (
		          SELECT 1 FROM erp.gl_voucher_lines l
		          WHERE l.tenant_id = erp.gl_accounts.tenant_id AND l.account_id = erp.gl_accounts.id
		      )
		  )
		ORDER BY code`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list report accounts: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Account, 0)
	for rows.Next() {
		item, scanErr := scanAccount(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *ReportRepository) getLeafAccount(ctx context.Context, tenantID, id string) (*domain.Account, error) {
	row := r.db.QueryRow(ctx, accountSelectSQL+`
		WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return scanAccount(row)
}
