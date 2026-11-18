package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type cashFlowQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *ReportRepository) cashCodes(ctx context.Context, q cashFlowQuerier, tenantID string) ([]string, error) {
	rows, err := q.Query(ctx, `
		SELECT key, account_code
		FROM erp.gl_account_mappings
		WHERE tenant_id = $1 AND key IN ('cash', 'bank')
		ORDER BY key, account_code`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("load cash mappings: %w", err)
	}
	defer rows.Close()
	mappings := make([]struct{ Key, Code string }, 0, 4)
	for rows.Next() {
		var mapping struct{ Key, Code string }
		if err := rows.Scan(&mapping.Key, &mapping.Code); err != nil {
			return nil, fmt.Errorf("scan cash mapping: %w", err)
		}
		mappings = append(mappings, mapping)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cash mappings: %w", err)
	}
	return buildCashCodeSet(mappings), nil
}

// buildCashCodeSet expands mapping rows into exact codes plus numeric roots and
// fills in the default cash and bank roots when either key is absent.
func buildCashCodeSet(mappings []struct{ Key, Code string }) []string {
	codes := make([]string, 0, len(mappings)*2+2)
	seen := make(map[string]bool, len(mappings)*2+2)
	add := func(code string) {
		code = strings.TrimSpace(code)
		if code == "" || seen[code] {
			return
		}
		seen[code] = true
		codes = append(codes, code)
	}

	hasCash := false
	hasBank := false
	for _, mapping := range mappings {
		code := strings.TrimSpace(mapping.Code)
		if code == "" {
			continue
		}
		switch mapping.Key {
		case "cash":
			hasCash = true
		case "bank":
			hasBank = true
		}
		add(code)
		if root := domain.CashFlowAccountRoot(code); root != "" {
			add(root)
		}
	}
	if !hasCash {
		add("1001")
	}
	if !hasBank {
		add("1002")
	}
	return codes
}

func (r *ReportRepository) CashFlowStatement(
	ctx context.Context,
	filter scope.Filter,
	tenantID string,
	startDate, endDate time.Time,
) (*domain.CashFlowStatement, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin cash flow snapshot: %w", err)
	}
	defer tx.Rollback(ctx)

	codes, err := r.cashCodes(ctx, tx, tenantID)
	if err != nil {
		return nil, err
	}
	cashSet := make(map[string]bool, len(codes))
	for _, code := range codes {
		cashSet[code] = true
	}

	args := []any{tenantID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")}
	where := "l.tenant_id = $1 AND v.status IN ('posted', 'reversed') AND v.voucher_date BETWEEN $2::date AND $3::date"
	appendScope(&where, &args, "v", filter)
	rows, err := tx.Query(ctx, `
		SELECT v.id::text, v.code, v.voucher_date, a.code, l.debit::text, l.credit::text
		FROM erp.gl_voucher_lines l
		JOIN erp.gl_vouchers v ON v.tenant_id = l.tenant_id AND v.id = l.voucher_id
		JOIN erp.gl_accounts a ON a.tenant_id = l.tenant_id AND a.id = l.account_id
		WHERE `+where+`
		ORDER BY v.voucher_date, v.code, l.line_no`, args...)
	if err != nil {
		return nil, fmt.Errorf("list cash flow vouchers: %w", err)
	}
	defer rows.Close()

	voucherIndex := map[string]int{}
	vouchers := make([]domain.CashFlowVoucher, 0)
	for rows.Next() {
		var voucherID, voucherCode, accountCode, debitRaw, creditRaw string
		var voucherDate time.Time
		if err := rows.Scan(&voucherID, &voucherCode, &voucherDate, &accountCode, &debitRaw, &creditRaw); err != nil {
			return nil, fmt.Errorf("scan cash flow line: %w", err)
		}
		index, ok := voucherIndex[voucherID]
		if !ok {
			index = len(vouchers)
			voucherIndex[voucherID] = index
			vouchers = append(vouchers, domain.CashFlowVoucher{
				VoucherID: voucherID, VoucherCode: voucherCode, VoucherDate: voucherDate,
			})
		}
		vouchers[index].Lines = append(vouchers[index].Lines, domain.CashFlowVoucherLine{
			AccountCode: accountCode,
			Debit:       decimal.RequireFromString(debitRaw),
			Credit:      decimal.RequireFromString(creditRaw),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cash flow vouchers: %w", err)
	}

	itemDefs := domain.CashFlowItemDefs()
	totals := make(map[string]decimal.Decimal, len(itemDefs))
	for _, item := range itemDefs {
		totals[item.Code] = decimal.Zero
	}
	unclassified := make([]*domain.CashFlowUnclassified, 0)
	for _, voucher := range vouchers {
		for _, allocation := range domain.AllocateCashFlow(voucher, cashSet) {
			if allocation.Amount.IsZero() {
				continue
			}
			item, ok := domain.CashFlowItem(allocation.ItemCode)
			if !ok || item.Direction != allocation.Direction {
				return nil, fmt.Errorf("unknown or inconsistent cash flow item %q", allocation.ItemCode)
			}
			totals[allocation.ItemCode] = totals[allocation.ItemCode].Add(allocation.Amount)
			if allocation.Unclassified {
				unclassified = append(unclassified, &domain.CashFlowUnclassified{
					VoucherID: allocation.VoucherID, VoucherCode: allocation.VoucherCode,
					VoucherDate: allocation.VoucherDate, Direction: allocation.Direction,
					Amount: allocation.Amount.Round(4), Reason: allocation.Reason,
				})
			}
		}
	}

	operating := buildCashFlowSection(domain.CashFlowOperating, totals)
	investing := buildCashFlowSection(domain.CashFlowInvesting, totals)
	financing := buildCashFlowSection(domain.CashFlowFinancing, totals)
	netIncrease := operating.Net.Add(investing.Net).Add(financing.Net)

	openingCash, err := r.cashBalance(ctx, tx, filter, tenantID, codes, startDate.AddDate(0, 0, -1))
	if err != nil {
		return nil, err
	}
	closingCash, err := r.cashBalance(ctx, tx, filter, tenantID, codes, endDate)
	if err != nil {
		return nil, err
	}

	income, err := r.incomeStatement(ctx, tx, filter, tenantID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	nonCash, err := r.nonCashExpenses(ctx, tx, filter, tenantID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	arIncrease, err := r.balanceChange(ctx, tx, filter, tenantID, []string{"1122"}, startDate, endDate, false)
	if err != nil {
		return nil, err
	}
	inventoryIncrease, err := r.balanceChange(ctx, tx, filter, tenantID, []string{"1405"}, startDate, endDate, false)
	if err != nil {
		return nil, err
	}
	apIncrease, err := r.balanceChange(ctx, tx, filter, tenantID, []string{"2202"}, startDate, endDate, true)
	if err != nil {
		return nil, err
	}
	taxIncrease, err := r.balanceChange(ctx, tx, filter, tenantID, []string{"2221"}, startDate, endDate, true)
	if err != nil {
		return nil, err
	}
	employeeIncrease, err := r.balanceChange(ctx, tx, filter, tenantID, []string{"2211"}, startDate, endDate, true)
	if err != nil {
		return nil, err
	}
	indirectNet := income.NetProfit.Add(nonCash).
		Sub(arIncrease).Sub(inventoryIncrease).
		Add(apIncrease).Add(taxIncrease).Add(employeeIncrease)

	return &domain.CashFlowStatement{
		StartDate: startDate, EndDate: endDate,
		Operating: operating, Investing: investing, Financing: financing,
		NetIncrease: netIncrease.Round(4), OpeningCash: openingCash.Round(4), ClosingCash: closingCash.Round(4),
		Unclassified: unclassified,
		Indirect: domain.IndirectCashFlow{
			NetProfit: income.NetProfit.Round(4), NonCashExpenses: nonCash.Round(4),
			ReceivableIncrease: arIncrease.Round(4), InventoryIncrease: inventoryIncrease.Round(4),
			PayableIncrease: apIncrease.Round(4), TaxPayableIncrease: taxIncrease.Round(4),
			EmployeePayableIncrease: employeeIncrease.Round(4),
			OperatingNet:            indirectNet.Round(4), DirectOperatingNet: operating.Net.Round(4),
			Difference: indirectNet.Sub(operating.Net).Round(4),
		},
	}, nil
}

func buildCashFlowSection(category string, totals map[string]decimal.Decimal) domain.CashFlowSection {
	section := domain.CashFlowSection{
		Inflows:  make([]*domain.CashFlowLine, 0),
		Outflows: make([]*domain.CashFlowLine, 0),
	}
	for _, item := range domain.CashFlowItemDefs() {
		if item.Category != category {
			continue
		}
		line := &domain.CashFlowLine{Code: item.Code, Name: item.Name, Amount: totals[item.Code].Round(4)}
		if item.Direction == domain.CashFlowInflow {
			section.Inflows = append(section.Inflows, line)
			section.Net = section.Net.Add(line.Amount)
		} else {
			section.Outflows = append(section.Outflows, line)
			section.Net = section.Net.Sub(line.Amount)
		}
	}
	section.Net = section.Net.Round(4)
	return section
}

func (r *ReportRepository) cashBalance(
	ctx context.Context, q cashFlowQuerier, filter scope.Filter, tenantID string, codes []string, asOf time.Time,
) (decimal.Decimal, error) {
	args := []any{tenantID, codes, asOf.Format("2006-01-02")}
	where := "l.tenant_id = $1 AND (a.code = ANY($2::text[]) OR left(a.code, 4) = ANY($2::text[])) AND v.status IN ('posted', 'reversed') AND v.voucher_date <= $3::date"
	appendScope(&where, &args, "v", filter)
	var raw string
	if err := q.QueryRow(ctx, `
		SELECT COALESCE(SUM(l.debit - l.credit), 0)::text
		FROM erp.gl_voucher_lines l
		JOIN erp.gl_vouchers v ON v.tenant_id = l.tenant_id AND v.id = l.voucher_id
		JOIN erp.gl_accounts a ON a.tenant_id = l.tenant_id AND a.id = l.account_id
		WHERE `+where, args...).Scan(&raw); err != nil {
		return decimal.Zero, fmt.Errorf("cash balance: %w", err)
	}
	return decimal.RequireFromString(raw), nil
}

func (r *ReportRepository) nonCashExpenses(
	ctx context.Context, q cashFlowQuerier, filter scope.Filter, tenantID string, startDate, endDate time.Time,
) (decimal.Decimal, error) {
	args := []any{tenantID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")}
	where := "l.tenant_id = $1 AND v.status IN ('posted', 'reversed') AND v.voucher_date BETWEEN $2::date AND $3::date AND v.voucher_type NOT IN ('period_close', 'period_close_reversal', 'year_close')"
	appendScope(&where, &args, "v", filter)
	var raw string
	if err := q.QueryRow(ctx, `
		SELECT COALESCE(SUM(l.debit - l.credit), 0)::text
		FROM erp.gl_voucher_lines l
		JOIN erp.gl_vouchers v ON v.tenant_id = l.tenant_id AND v.id = l.voucher_id
		JOIN erp.gl_accounts a ON a.tenant_id = l.tenant_id AND a.id = l.account_id
		JOIN erp.gl_account_mappings m
		  ON m.tenant_id = l.tenant_id AND m.key = 'non_cash_expense'
		 AND (m.account_code = a.code OR m.account_code = left(a.code, 4))
		WHERE `+where, args...).Scan(&raw); err != nil {
		return decimal.Zero, fmt.Errorf("non cash expenses: %w", err)
	}
	return decimal.RequireFromString(raw), nil
}

// balanceChange excludes close vouchers for indirect working capital, matching income statement scope.
func (r *ReportRepository) balanceChange(
	ctx context.Context, q cashFlowQuerier, filter scope.Filter, tenantID string, codes []string,
	startDate, endDate time.Time, liability bool,
) (decimal.Decimal, error) {
	args := []any{tenantID, codes, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")}
	where := "l.tenant_id = $1 AND (a.code = ANY($2::text[]) OR left(a.code, 4) = ANY($2::text[])) AND v.status IN ('posted', 'reversed') AND v.voucher_type NOT IN ('period_close', 'period_close_reversal', 'year_close') AND v.voucher_date <= $4::date"
	appendScope(&where, &args, "v", filter)
	var openingRaw, closingRaw string
	if err := q.QueryRow(ctx, `
		SELECT
		  COALESCE(SUM(CASE WHEN v.voucher_date < $3::date THEN l.debit - l.credit ELSE 0 END), 0)::text,
		  COALESCE(SUM(l.debit - l.credit), 0)::text
		FROM erp.gl_voucher_lines l
		JOIN erp.gl_vouchers v ON v.tenant_id = l.tenant_id AND v.id = l.voucher_id
		JOIN erp.gl_accounts a ON a.tenant_id = l.tenant_id AND a.id = l.account_id
		WHERE `+where, args...).Scan(&openingRaw, &closingRaw); err != nil {
		return decimal.Zero, fmt.Errorf("account balance change: %w", err)
	}
	opening := decimal.RequireFromString(openingRaw)
	closing := decimal.RequireFromString(closingRaw)
	change := closing.Sub(opening)
	if liability {
		change = change.Neg()
	}
	return change.Round(4), nil
}
