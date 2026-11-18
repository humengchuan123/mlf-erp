package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

func (r *ReportRepository) cashCodes(ctx context.Context, tenantID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT account_code
		FROM erp.gl_account_mappings
		WHERE tenant_id = $1 AND key IN ('cash', 'bank')
		ORDER BY account_code`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("load cash mappings: %w", err)
	}
	defer rows.Close()
	codes := make([]string, 0, 2)
	seen := map[string]bool{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan cash mapping: %w", err)
		}
		root := domain.CashFlowAccountRoot(code)
		if root == "" || seen[root] {
			continue
		}
		seen[root] = true
		codes = append(codes, root)
	}
	if len(codes) == 0 {
		return []string{"1001", "1002"}, nil
	}
	return codes, rows.Err()
}

func (r *ReportRepository) CashFlowStatement(
	ctx context.Context,
	filter scope.Filter,
	tenantID string,
	startDate, endDate time.Time,
) (*domain.CashFlowStatement, error) {
	codes, err := r.cashCodes(ctx, tenantID)
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
	rows, err := r.db.Query(ctx, `
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
		return nil, err
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
			if allocation.ItemCode == "" {
				return nil, fmt.Errorf("unknown cash flow item %q", allocation.ItemCode)
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

	openingCash, err := r.cashBalance(ctx, filter, tenantID, codes, startDate.AddDate(0, 0, -1))
	if err != nil {
		return nil, err
	}
	closingCash, err := r.cashBalance(ctx, filter, tenantID, codes, endDate)
	if err != nil {
		return nil, err
	}

	income, err := r.IncomeStatement(ctx, filter, tenantID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	nonCash, err := r.nonCashExpenses(ctx, filter, tenantID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	arIncrease, err := r.balanceChange(ctx, filter, tenantID, []string{"1122"}, startDate, endDate, false)
	if err != nil {
		return nil, err
	}
	inventoryIncrease, err := r.balanceChange(ctx, filter, tenantID, []string{"1405"}, startDate, endDate, false)
	if err != nil {
		return nil, err
	}
	apIncrease, err := r.balanceChange(ctx, filter, tenantID, []string{"2202"}, startDate, endDate, true)
	if err != nil {
		return nil, err
	}
	taxIncrease, err := r.balanceChange(ctx, filter, tenantID, []string{"2221"}, startDate, endDate, true)
	if err != nil {
		return nil, err
	}
	employeeIncrease, err := r.balanceChange(ctx, filter, tenantID, []string{"2211"}, startDate, endDate, true)
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
	ctx context.Context, filter scope.Filter, tenantID string, codes []string, asOf time.Time,
) (decimal.Decimal, error) {
	args := []any{tenantID, codes, asOf.Format("2006-01-02")}
	where := "l.tenant_id = $1 AND (a.code = ANY($2::text[]) OR left(a.code, 4) = ANY($2::text[])) AND v.status IN ('posted', 'reversed') AND v.voucher_date <= $3::date"
	appendScope(&where, &args, "v", filter)
	var raw string
	if err := r.db.QueryRow(ctx, `
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
	ctx context.Context, filter scope.Filter, tenantID string, startDate, endDate time.Time,
) (decimal.Decimal, error) {
	args := []any{tenantID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")}
	where := "l.tenant_id = $1 AND v.status IN ('posted', 'reversed') AND v.voucher_date BETWEEN $2::date AND $3::date"
	appendScope(&where, &args, "v", filter)
	var raw string
	if err := r.db.QueryRow(ctx, `
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

func (r *ReportRepository) balanceChange(
	ctx context.Context, filter scope.Filter, tenantID string, codes []string,
	startDate, endDate time.Time, liability bool,
) (decimal.Decimal, error) {
	args := []any{tenantID, codes, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")}
	where := "l.tenant_id = $1 AND (a.code = ANY($2::text[]) OR left(a.code, 4) = ANY($2::text[])) AND v.status IN ('posted', 'reversed') AND v.voucher_date <= $4::date"
	appendScope(&where, &args, "v", filter)
	var openingRaw, closingRaw string
	if err := r.db.QueryRow(ctx, `
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
