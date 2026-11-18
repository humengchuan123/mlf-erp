# Stage 3G 现金流量表 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 基于现有总账凭证生成直接法现金流量表主表和标准简化间接法调节表。

**Architecture:** 在 GL 域新增现金流项目定义和分配算法；Repository 查询现金/银行凭证及对方科目，按比例分摊并汇总；HTTP/SDK/OpenAPI/前端增加报表页签；API 测试覆盖经营、投资、筹资、互转、未分类和间接法。

**Tech Stack:** Go 1.25 + Gin + pgx + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus。

**Design:** `docs/superpowers/specs/2026-11-18-stage-3g-cash-flow-design.md`

---

## Task 1: 现金流域模型、科目映射和分摊算法

**Files:**
- Create: `internal/modules/gl/domain/cashflow.go`
- Create: `internal/modules/gl/domain/cashflow_test.go`

- [ ] **Step 1: 写失败测试**

```go
package domain

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestClassifyCashFlow(t *testing.T) {
	tests := []struct {
		code          string
		inflow        bool
		wantCode      string
		wantUnclassified bool
	}{
		{"1122", true, "sales_receipts", false},
		{"1122", false, "other_operating_outflows", false},
		{"2202", false, "purchase_payments", false},
		{"1405", false, "purchase_payments", false},
		{"2211", false, "employee_payments", false},
		{"2221", false, "tax_payments", false},
		{"2221", true, "tax_refunds", false},
		{"1601", false, "asset_purchases", false},
		{"1601", true, "disposal_asset_receipts", false},
		{"4001", true, "capital_injections", false},
		{"4104", false, "dividend_interest_payments", false},
		{"9999", true, "other_operating_inflows", true},
	}
	for _, tt := range tests {
		item, classified := ClassifyCashFlow(tt.code, tt.inflow)
		if item.Code != tt.wantCode {
			t.Fatalf("%s inflow=%v code = %s, want %s", tt.code, tt.inflow, item.Code, tt.wantCode)
		}
		if classified != !tt.wantUnclassified {
			t.Fatalf("%s inflow=%v classified = %v, want unclassified=%v", tt.code, tt.inflow, classified, tt.wantUnclassified)
		}
	}
}

func TestAllocateCashFlowSingleCounterpart(t *testing.T) {
	voucher := CashFlowVoucher{
		VoucherID: "v1", VoucherCode: "GL-1", VoucherDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		Lines: []CashFlowVoucherLine{
			{AccountCode: "1002", Debit: decimal.NewFromInt(100)},
			{AccountCode: "1122", Credit: decimal.NewFromInt(100)},
		},
	}
	allocations := AllocateCashFlow(voucher, map[string]bool{"1001": true, "1002": true})
	if len(allocations) != 1 {
		t.Fatalf("allocations = %d, want 1", len(allocations))
	}
	if allocations[0].ItemCode != "sales_receipts" || !allocations[0].Amount.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("allocation = %+v, want sales_receipts 100", allocations[0])
	}
}

func TestAllocateCashFlowMultipleCounterpartsAndRounding(t *testing.T) {
	voucher := CashFlowVoucher{
		VoucherID: "v2", VoucherCode: "GL-2", VoucherDate: time.Now(),
		Lines: []CashFlowVoucherLine{
			{AccountCode: "1002", Debit: decimal.RequireFromString("10")},
			{AccountCode: "1122", Credit: decimal.RequireFromString("1")},
			{AccountCode: "1122", Credit: decimal.RequireFromString("2")},
		},
	}
	allocations := AllocateCashFlow(voucher, map[string]bool{"1002": true})
	total := decimal.Zero
	for _, allocation := range allocations {
		total = total.Add(allocation.Amount)
	}
	if !total.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("allocated total = %s, want 10", total)
	}
	if !allocations[0].Amount.Equal(decimal.RequireFromString("3.3333")) {
		t.Fatalf("first amount = %s, want 3.3333", allocations[0].Amount)
	}
	if !allocations[1].Amount.Equal(decimal.RequireFromString("6.6667")) {
		t.Fatalf("last amount = %s, want 6.6667", allocations[1].Amount)
	}
}

func TestAllocateCashFlowIgnoresInternalTransfer(t *testing.T) {
	voucher := CashFlowVoucher{
		VoucherID: "v3", VoucherCode: "GL-3", VoucherDate: time.Now(),
		Lines: []CashFlowVoucherLine{
			{AccountCode: "1001", Debit: decimal.NewFromInt(25)},
			{AccountCode: "1002", Credit: decimal.NewFromInt(25)},
		},
	}
	if got := AllocateCashFlow(voucher, map[string]bool{"1001": true, "1002": true}); len(got) != 0 {
		t.Fatalf("internal transfer allocations = %d, want 0", len(got))
	}
}

func TestAllocateCashFlowUnclassified(t *testing.T) {
	voucher := CashFlowVoucher{
		VoucherID: "v4", VoucherCode: "GL-4", VoucherDate: time.Now(),
		Lines: []CashFlowVoucherLine{
			{AccountCode: "1002", Debit: decimal.NewFromInt(5)},
			{AccountCode: "4103", Credit: decimal.NewFromInt(5)},
		},
	}
	allocations := AllocateCashFlow(voucher, map[string]bool{"1002": true})
	if len(allocations) != 1 || !allocations[0].Unclassified {
		t.Fatalf("allocation = %+v, want one unclassified allocation", allocations)
	}
	if allocations[0].ItemCode != "other_operating_inflows" {
		t.Fatalf("item = %s, want other_operating_inflows", allocations[0].ItemCode)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/gl/domain -run 'TestClassifyCashFlow|TestAllocateCashFlow' -v`

Expected: FAIL，`ClassifyCashFlow` / `AllocateCashFlow` 未定义。

- [ ] **Step 3: 实现域模型**

```go
package domain

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const (
	CashFlowOperating = "operating"
	CashFlowInvesting = "investing"
	CashFlowFinancing = "financing"

	CashFlowInflow  = "inflow"
	CashFlowOutflow = "outflow"
)

type CashFlowItemDef struct {
	Code      string
	Name      string
	Category  string
	Direction string
}

var CashFlowItems = []CashFlowItemDef{
	{Code: "sales_receipts", Name: "销售商品、提供劳务收到的现金", Category: CashFlowOperating, Direction: CashFlowInflow},
	{Code: "tax_refunds", Name: "收到的税费返还", Category: CashFlowOperating, Direction: CashFlowInflow},
	{Code: "other_operating_inflows", Name: "收到其他与经营活动有关的现金", Category: CashFlowOperating, Direction: CashFlowInflow},
	{Code: "purchase_payments", Name: "购买商品、接受劳务支付的现金", Category: CashFlowOperating, Direction: CashFlowOutflow},
	{Code: "employee_payments", Name: "支付给职工以及为职工支付的现金", Category: CashFlowOperating, Direction: CashFlowOutflow},
	{Code: "tax_payments", Name: "支付的各项税费", Category: CashFlowOperating, Direction: CashFlowOutflow},
	{Code: "other_operating_outflows", Name: "支付其他与经营活动有关的现金", Category: CashFlowOperating, Direction: CashFlowOutflow},
	{Code: "disposal_asset_receipts", Name: "处置固定资产、无形资产和其他长期资产收回的现金净额", Category: CashFlowInvesting, Direction: CashFlowInflow},
	{Code: "other_investing_inflows", Name: "收到其他与投资活动有关的现金", Category: CashFlowInvesting, Direction: CashFlowInflow},
	{Code: "asset_purchases", Name: "购建固定资产、无形资产和其他长期资产支付的现金", Category: CashFlowInvesting, Direction: CashFlowOutflow},
	{Code: "investment_payments", Name: "投资支付的现金", Category: CashFlowInvesting, Direction: CashFlowOutflow},
	{Code: "other_investing_outflows", Name: "支付其他与投资活动有关的现金", Category: CashFlowInvesting, Direction: CashFlowOutflow},
	{Code: "capital_injections", Name: "吸收投资收到的现金", Category: CashFlowFinancing, Direction: CashFlowInflow},
	{Code: "borrowings", Name: "取得借款收到的现金", Category: CashFlowFinancing, Direction: CashFlowInflow},
	{Code: "other_financing_inflows", Name: "收到其他与筹资活动有关的现金", Category: CashFlowFinancing, Direction: CashFlowInflow},
	{Code: "debt_repayments", Name: "偿还债务支付的现金", Category: CashFlowFinancing, Direction: CashFlowOutflow},
	{Code: "dividend_interest_payments", Name: "分配股利、利润或偿付利息支付的现金", Category: CashFlowFinancing, Direction: CashFlowOutflow},
	{Code: "other_financing_outflows", Name: "支付其他与筹资活动有关的现金", Category: CashFlowFinancing, Direction: CashFlowOutflow},
}

var cashFlowItemMap = func() map[string]CashFlowItemDef {
	items := make(map[string]CashFlowItemDef, len(CashFlowItems))
	for _, item := range CashFlowItems {
		items[item.Code] = item
	}
	return items
}()

type CashFlowLine struct {
	Code   string
	Name   string
	Amount decimal.Decimal
}

type CashFlowSection struct {
	Inflows  []*CashFlowLine
	Outflows []*CashFlowLine
	Net      decimal.Decimal
}

type CashFlowUnclassified struct {
	VoucherID   string
	VoucherCode string
	VoucherDate time.Time
	Direction   string
	Amount      decimal.Decimal
	Reason      string
}

type IndirectCashFlow struct {
	NetProfit              decimal.Decimal
	NonCashExpenses        decimal.Decimal
	ReceivableIncrease     decimal.Decimal
	InventoryIncrease      decimal.Decimal
	PayableIncrease        decimal.Decimal
	TaxPayableIncrease     decimal.Decimal
	EmployeePayableIncrease decimal.Decimal
	OperatingNet           decimal.Decimal
	DirectOperatingNet     decimal.Decimal
	Difference             decimal.Decimal
}

type CashFlowStatement struct {
	StartDate    time.Time
	EndDate      time.Time
	Operating    CashFlowSection
	Investing    CashFlowSection
	Financing    CashFlowSection
	NetIncrease  decimal.Decimal
	OpeningCash  decimal.Decimal
	ClosingCash  decimal.Decimal
	Unclassified []*CashFlowUnclassified
	Indirect     IndirectCashFlow
}

type CashFlowVoucherLine struct {
	AccountCode string
	Debit       decimal.Decimal
	Credit      decimal.Decimal
}

type CashFlowVoucher struct {
	VoucherID   string
	VoucherCode string
	VoucherDate time.Time
	Lines       []CashFlowVoucherLine
}

type CashFlowAllocation struct {
	VoucherID    string
	VoucherCode  string
	VoucherDate  time.Time
	ItemCode     string
	Category     string
	Direction    string
	Amount       decimal.Decimal
	Unclassified bool
	Reason       string
}

func CashFlowItem(code string) CashFlowItemDef {
	return cashFlowItemMap[code]
}

func ClassifyCashFlow(accountCode string, inflow bool) (CashFlowItemDef, bool) {
	code := strings.TrimSpace(accountCode)
	fallback := "other_operating_inflows"
	if !inflow {
		fallback = "other_operating_outflows"
	}
	switch {
	case code == "1122":
		if inflow {
			return CashFlowItem("sales_receipts"), true
		}
		return CashFlowItem("other_operating_outflows"), true
	case code == "2202" || code == "1405":
		if inflow {
			return CashFlowItem("other_operating_inflows"), true
		}
		return CashFlowItem("purchase_payments"), true
	case code == "2211":
		if inflow {
			return CashFlowItem("other_operating_inflows"), true
		}
		return CashFlowItem("employee_payments"), true
	case strings.HasPrefix(code, "2221"):
		if inflow {
			return CashFlowItem("tax_refunds"), true
		}
		return CashFlowItem("tax_payments"), true
	case strings.HasPrefix(code, "6001") || strings.HasPrefix(code, "6051"):
		if inflow {
			return CashFlowItem("sales_receipts"), true
		}
		return CashFlowItem("other_operating_outflows"), true
	case strings.HasPrefix(code, "6401") || strings.HasPrefix(code, "6601") ||
		strings.HasPrefix(code, "6602") || strings.HasPrefix(code, "6603") ||
		strings.HasPrefix(code, "6801"):
		if inflow {
			return CashFlowItem("other_operating_inflows"), true
		}
		return CashFlowItem("other_operating_outflows"), true
	case strings.HasPrefix(code, "16") || strings.HasPrefix(code, "17"):
		if inflow {
			return CashFlowItem("disposal_asset_receipts"), true
		}
		return CashFlowItem("asset_purchases"), true
	case strings.HasPrefix(code, "2001") || strings.HasPrefix(code, "2501"):
		if inflow {
			return CashFlowItem("borrowings"), true
		}
		return CashFlowItem("debt_repayments"), true
	case strings.HasPrefix(code, "4001"):
		if inflow {
			return CashFlowItem("capital_injections"), true
		}
		return CashFlowItem("other_financing_outflows"), true
	case strings.HasPrefix(code, "4104"):
		if inflow {
			return CashFlowItem("other_financing_inflows"), true
		}
		return CashFlowItem("dividend_interest_payments"), true
	default:
		return CashFlowItem(fallback), false
	}
}

func AllocateCashFlow(voucher CashFlowVoucher, cashCodes map[string]bool) []CashFlowAllocation {
	cashIn := decimal.Zero
	cashOut := decimal.Zero
	credits := make([]CashFlowVoucherLine, 0)
	debits := make([]CashFlowVoucherLine, 0)
	for _, line := range voucher.Lines {
		if cashCodes[line.AccountCode] {
			cashIn = cashIn.Add(line.Debit)
			cashOut = cashOut.Add(line.Credit)
			continue
		}
		if line.Credit.IsPositive() {
			credits = append(credits, line)
		}
		if line.Debit.IsPositive() {
			debits = append(debits, line)
		}
	}
	if cashIn.IsZero() && cashOut.IsZero() {
		return nil
	}
	if cashIn.Equal(cashOut) && len(credits) == 0 && len(debits) == 0 {
		return nil
	}
	allocations := make([]CashFlowAllocation, 0)
	if cashIn.IsPositive() {
		allocations = append(allocations, allocateCashFlowSide(voucher, cashIn, credits, true)...)
	}
	if cashOut.IsPositive() {
		allocations = append(allocations, allocateCashFlowSide(voucher, cashOut, debits, false)...)
	}
	return allocations
}

func allocateCashFlowSide(voucher CashFlowVoucher, amount decimal.Decimal, counterparts []CashFlowVoucherLine, inflow bool) []CashFlowAllocation {
	if len(counterparts) == 0 {
		item := CashFlowItem("other_operating_inflows")
		if !inflow {
			item = CashFlowItem("other_operating_outflows")
		}
		return []CashFlowAllocation{{
			VoucherID: voucher.VoucherID, VoucherCode: voucher.VoucherCode, VoucherDate: voucher.VoucherDate,
			ItemCode: item.Code, Category: item.Category, Direction: item.Direction,
			Amount: amount.Round(4), Unclassified: true, Reason: "无对方科目",
		}}
	}
	total := decimal.Zero
	for _, line := range counterparts {
		if inflow {
			total = total.Add(line.Credit)
		} else {
			total = total.Add(line.Debit)
		}
	}
	if !total.IsPositive() {
		return nil
	}
	remaining := amount
	allocations := make([]CashFlowAllocation, 0, len(counterparts))
	for i, line := range counterparts {
		weight := line.Credit
		if !inflow {
			weight = line.Debit
		}
		allocated := amount.Mul(weight).Div(total).Round(4)
		if i == len(counterparts)-1 {
			allocated = remaining.Round(4)
		}
		remaining = remaining.Sub(allocated)
		item, classified := ClassifyCashFlow(line.AccountCode, inflow)
		reason := ""
		if !classified {
			reason = "未识别科目 " + line.AccountCode
		}
		allocations = append(allocations, CashFlowAllocation{
			VoucherID: voucher.VoucherID, VoucherCode: voucher.VoucherCode, VoucherDate: voucher.VoucherDate,
			ItemCode: item.Code, Category: item.Category, Direction: item.Direction,
			Amount: allocated, Unclassified: !classified, Reason: reason,
		})
	}
	return allocations
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/gl/domain -run 'TestClassifyCashFlow|TestAllocateCashFlow' -v`

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/gl/domain/cashflow.go internal/modules/gl/domain/cashflow_test.go
git commit -m "feat(stage-3g): add cash flow domain model"
```

---

## Task 2: Repository 查询、分摊汇总和间接法

**Files:**
- Modify: `internal/modules/gl/domain/repository.go`
- Create: `internal/modules/gl/infra/postgres/cashflow_repository.go`

- [ ] **Step 1: 扩展 ReportRepository 接口**

```go
type ReportRepository interface {
	TrialBalance(ctx context.Context, filter scope.Filter, tenantID string, startDate, endDate time.Time) (*TrialBalance, error)
	GeneralLedger(ctx context.Context, filter scope.Filter, tenantID, accountID string, startDate, endDate time.Time) (*GeneralLedger, error)
	BalanceSheet(ctx context.Context, filter scope.Filter, tenantID string, asOf time.Time) (*BalanceSheet, error)
	IncomeStatement(ctx context.Context, filter scope.Filter, tenantID string, startDate, endDate time.Time) (*IncomeStatement, error)
	AuxiliaryBalance(ctx context.Context, filter scope.Filter, tenantID, dimension, accountID string, startDate, endDate time.Time) (*AuxiliaryBalance, error)
	VATSummary(ctx context.Context, filter scope.Filter, tenantID string, startDate, endDate time.Time) (*VATSummary, error)
	CashFlowStatement(ctx context.Context, filter scope.Filter, tenantID string, startDate, endDate time.Time) (*CashFlowStatement, error)
}
```

- [ ] **Step 2: 实现现金账户查询**

```go
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
		if !seen[code] {
			seen[code] = true
			codes = append(codes, code)
		}
	}
	if len(codes) == 0 {
		return []string{"1001", "1002"}, nil
	}
	return codes, rows.Err()
}
```

- [ ] **Step 3: 实现凭证查询和汇总**

```go
func (r *ReportRepository) CashFlowStatement(
	ctx context.Context,
	filter scope.Filter,
	tenantID string,
	startDate, endDate time.Time,
) (*CashFlowStatement, error) {
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
	vouchers := make([]CashFlowVoucher, 0)
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
			vouchers = append(vouchers, CashFlowVoucher{
				VoucherID: voucherID, VoucherCode: voucherCode, VoucherDate: voucherDate,
			})
		}
		vouchers[index].Lines = append(vouchers[index].Lines, CashFlowVoucherLine{
			AccountCode: accountCode,
			Debit:       decimal.RequireFromString(debitRaw),
			Credit:      decimal.RequireFromString(creditRaw),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	totals := make(map[string]decimal.Decimal, len(CashFlowItems))
	for _, item := range CashFlowItems {
		totals[item.Code] = decimal.Zero
	}
	unclassified := make([]*CashFlowUnclassified, 0)
	for _, voucher := range vouchers {
		for _, allocation := range AllocateCashFlow(voucher, cashSet) {
			totals[allocation.ItemCode] = totals[allocation.ItemCode].Add(allocation.Amount)
			if allocation.Unclassified {
				unclassified = append(unclassified, &CashFlowUnclassified{
					VoucherID: allocation.VoucherID, VoucherCode: allocation.VoucherCode,
					VoucherDate: allocation.VoucherDate, Direction: allocation.Direction,
					Amount: allocation.Amount.Round(4), Reason: allocation.Reason,
				})
			}
		}
	}

	operating := buildCashFlowSection(CashFlowOperating, totals)
	investing := buildCashFlowSection(CashFlowInvesting, totals)
	financing := buildCashFlowSection(CashFlowFinancing, totals)
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

	return &CashFlowStatement{
		StartDate: startDate, EndDate: endDate,
		Operating: operating, Investing: investing, Financing: financing,
		NetIncrease: netIncrease.Round(4), OpeningCash: openingCash.Round(4), ClosingCash: closingCash.Round(4),
		Unclassified: unclassified,
		Indirect: IndirectCashFlow{
			NetProfit: income.NetProfit.Round(4), NonCashExpenses: nonCash.Round(4),
			ReceivableIncrease: arIncrease.Round(4), InventoryIncrease: inventoryIncrease.Round(4),
			PayableIncrease: apIncrease.Round(4), TaxPayableIncrease: taxIncrease.Round(4),
			EmployeePayableIncrease: employeeIncrease.Round(4),
			OperatingNet: indirectNet.Round(4), DirectOperatingNet: operating.Net.Round(4),
			Difference: indirectNet.Sub(operating.Net).Round(4),
		},
	}, nil
}

func buildCashFlowSection(category string, totals map[string]decimal.Decimal) CashFlowSection {
	section := CashFlowSection{Inflows: make([]*CashFlowLine, 0), Outflows: make([]*CashFlowLine, 0)}
	for _, item := range CashFlowItems {
		if item.Category != category {
			continue
		}
		line := &CashFlowLine{Code: item.Code, Name: item.Name, Amount: totals[item.Code].Round(4)}
		if item.Direction == CashFlowInflow {
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
```

- [ ] **Step 4: 实现余额辅助方法**

```go
func (r *ReportRepository) cashBalance(
	ctx context.Context, filter scope.Filter, tenantID string, codes []string, asOf time.Time,
) (decimal.Decimal, error) {
	args := []any{tenantID, codes, asOf.Format("2006-01-02")}
	where := "l.tenant_id = $1 AND a.code = ANY($2::text[]) AND v.status IN ('posted', 'reversed') AND v.voucher_date <= $3::date"
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
		  ON m.tenant_id = l.tenant_id AND m.account_code = a.code AND m.key = 'non_cash_expense'
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
	where := "l.tenant_id = $1 AND a.code = ANY($2::text[]) AND v.status IN ('posted', 'reversed') AND v.voucher_date <= $4::date"
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
```

- [ ] **Step 5: 编译**

Run: `go build ./...`

Expected: 成功。

- [ ] **Step 6: 提交**

```bash
git add internal/modules/gl/domain/repository.go internal/modules/gl/infra/postgres/cashflow_repository.go
git commit -m "feat(stage-3g): calculate cash flow statement"
```

---

## Task 3: 报表服务、HTTP 路由和响应

**Files:**
- Modify: `internal/modules/gl/app/report_service.go`
- Modify: `internal/modules/gl/http/handler.go`

- [ ] **Step 1: 增加服务方法**

```go
func (s *ReportService) CashFlowStatement(ctx context.Context, startDate, endDate time.Time) (*domain.CashFlowStatement, error) {
	tenantID, _, _ := tenantActor(ctx)
	startDate, endDate = normalizeRange(startDate, endDate)
	if startDate.After(endDate) {
		return nil, domain.ErrInvalidInput
	}
	return s.reports.CashFlowStatement(ctx, scope.FromActor(ctx, tenantID), tenantID, startDate, endDate)
}
```

- [ ] **Step 2: 增加路由**

在 `Register` 的报表路由中增加：

```go
finance.GET("/reports/cash-flow", authmw.RequirePermission("finance.report.read"), handler.cashFlowStatement)
```

- [ ] **Step 3: 增加响应 DTO 和 Handler**

```go
type cashFlowLineResponse struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Amount string `json:"amount"`
}

type cashFlowSectionResponse struct {
	Inflows  []cashFlowLineResponse `json:"inflows"`
	Outflows []cashFlowLineResponse `json:"outflows"`
	Net      string                 `json:"net"`
}

type cashFlowUnclassifiedResponse struct {
	VoucherID   string `json:"voucher_id"`
	VoucherCode string `json:"voucher_code"`
	VoucherDate string `json:"voucher_date"`
	Direction   string `json:"direction"`
	Amount      string `json:"amount"`
	Reason      string `json:"reason"`
}

type indirectCashFlowResponse struct {
	NetProfit               string `json:"net_profit"`
	NonCashExpenses         string `json:"non_cash_expenses"`
	ReceivableIncrease      string `json:"receivable_increase"`
	InventoryIncrease       string `json:"inventory_increase"`
	PayableIncrease         string `json:"payable_increase"`
	TaxPayableIncrease      string `json:"tax_payable_increase"`
	EmployeePayableIncrease string `json:"employee_payable_increase"`
	OperatingNet            string `json:"operating_net"`
	DirectOperatingNet      string `json:"direct_operating_net"`
	Difference              string `json:"difference"`
}

type cashFlowStatementResponse struct {
	StartDate    string                       `json:"start_date"`
	EndDate      string                       `json:"end_date"`
	Operating    cashFlowSectionResponse      `json:"operating"`
	Investing    cashFlowSectionResponse      `json:"investing"`
	Financing    cashFlowSectionResponse      `json:"financing"`
	NetIncrease  string                       `json:"net_increase"`
	OpeningCash  string                       `json:"opening_cash"`
	ClosingCash  string                       `json:"closing_cash"`
	Unclassified []cashFlowUnclassifiedResponse `json:"unclassified"`
	Indirect     indirectCashFlowResponse     `json:"indirect"`
}

func (h *Handler) cashFlowStatement(c *gin.Context) {
	startDate, endDate, err := parseRange(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid date range")
		return
	}
	report, err := h.reports.CashFlowStatement(c.Request.Context(), startDate, endDate)
	if err != nil {
		h.handleError(c, err)
		return
	}
	httpx.OK(c, cashFlowStatementResponse{
		StartDate: report.StartDate.Format("2006-01-02"), EndDate: report.EndDate.Format("2006-01-02"),
		Operating: toCashFlowSection(report.Operating), Investing: toCashFlowSection(report.Investing),
		Financing: toCashFlowSection(report.Financing),
		NetIncrease: report.NetIncrease.StringFixed(4), OpeningCash: report.OpeningCash.StringFixed(4),
		ClosingCash: report.ClosingCash.StringFixed(4),
		Unclassified: toCashFlowUnclassified(report.Unclassified),
		Indirect: indirectCashFlowResponse{
			NetProfit: report.Indirect.NetProfit.StringFixed(4),
			NonCashExpenses: report.Indirect.NonCashExpenses.StringFixed(4),
			ReceivableIncrease: report.Indirect.ReceivableIncrease.StringFixed(4),
			InventoryIncrease: report.Indirect.InventoryIncrease.StringFixed(4),
			PayableIncrease: report.Indirect.PayableIncrease.StringFixed(4),
			TaxPayableIncrease: report.Indirect.TaxPayableIncrease.StringFixed(4),
			EmployeePayableIncrease: report.Indirect.EmployeePayableIncrease.StringFixed(4),
			OperatingNet: report.Indirect.OperatingNet.StringFixed(4),
			DirectOperatingNet: report.Indirect.DirectOperatingNet.StringFixed(4),
			Difference: report.Indirect.Difference.StringFixed(4),
		},
	})
}

func toCashFlowSection(section domain.CashFlowSection) cashFlowSectionResponse {
	return cashFlowSectionResponse{
		Inflows: toCashFlowLines(section.Inflows), Outflows: toCashFlowLines(section.Outflows),
		Net: section.Net.StringFixed(4),
	}
}

func toCashFlowLines(lines []*domain.CashFlowLine) []cashFlowLineResponse {
	responses := make([]cashFlowLineResponse, 0, len(lines))
	for _, line := range lines {
		responses = append(responses, cashFlowLineResponse{
			Code: line.Code, Name: line.Name, Amount: line.Amount.StringFixed(4),
		})
	}
	return responses
}

func toCashFlowUnclassified(items []*domain.CashFlowUnclassified) []cashFlowUnclassifiedResponse {
	responses := make([]cashFlowUnclassifiedResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, cashFlowUnclassifiedResponse{
			VoucherID: item.VoucherID, VoucherCode: item.VoucherCode,
			VoucherDate: item.VoucherDate.Format("2006-01-02"), Direction: item.Direction,
			Amount: item.Amount.StringFixed(4), Reason: item.Reason,
		})
	}
	return responses
}
```

- [ ] **Step 4: 编译**

Run: `go build ./...`

Expected: 成功。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/gl/app/report_service.go internal/modules/gl/http/handler.go
git commit -m "feat(stage-3g): add cash flow report API"
```

---

## Task 4: SDK、OpenAPI 和前端完整度检查

**Files:**
- Modify: `packages/sdk/src/gl.ts`
- Modify: `api/openapi.yaml`
- Modify: `scripts/test/check-frontend.mjs`

- [ ] **Step 1: SDK 类型**

```ts
export interface CashFlowLine {
  code: string
  name: string
  amount: string
}

export interface CashFlowSection {
  inflows: CashFlowLine[]
  outflows: CashFlowLine[]
  net: string
}

export interface CashFlowUnclassified {
  voucher_id: string
  voucher_code: string
  voucher_date: string
  direction: string
  amount: string
  reason: string
}

export interface IndirectCashFlow {
  net_profit: string
  non_cash_expenses: string
  receivable_increase: string
  inventory_increase: string
  payable_increase: string
  tax_payable_increase: string
  employee_payable_increase: string
  operating_net: string
  direct_operating_net: string
  difference: string
}

export interface CashFlowStatement {
  start_date: string
  end_date: string
  operating: CashFlowSection
  investing: CashFlowSection
  financing: CashFlowSection
  net_increase: string
  opening_cash: string
  closing_cash: string
  unclassified: CashFlowUnclassified[]
  indirect: IndirectCashFlow
}
```

- [ ] **Step 2: SDK 方法**

```ts
cashFlow(params: { start_date?: string; end_date?: string } = {}): Promise<CashFlowStatement> {
  return api.get<CashFlowStatement>(`/finance/reports/cash-flow${queryString(params)}`)
},
```

- [ ] **Step 3: OpenAPI 路径和 schema**

先更新 `info.version` 为 `0.8.0`，描述增加 `Stage 3G`：

```yaml
  version: 0.8.0
  description: |
    MLF ERP Stage 1 + Stage 2 + Stage 3A/3B/3C/3D/3E/3F/3G API contract.
```

路径：

```yaml
  /finance/reports/cash-flow:
    get:
      summary: Cash flow statement
      parameters:
        - in: query
          name: start_date
          schema:
            type: string
            format: date
        - in: query
          name: end_date
          schema:
            type: string
            format: date
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                allOf:
                  - $ref: '#/components/schemas/ApiResponse'
                  - type: object
                    properties:
                      data:
                        $ref: '#/components/schemas/GlCashFlowStatement'
```

Schema：

```yaml
    GlCashFlowLine:
      type: object
      properties:
        code:
          type: string
        name:
          type: string
        amount:
          type: string
    GlCashFlowSection:
      type: object
      properties:
        inflows:
          type: array
          items:
            $ref: '#/components/schemas/GlCashFlowLine'
        outflows:
          type: array
          items:
            $ref: '#/components/schemas/GlCashFlowLine'
        net:
          type: string
    GlCashFlowUnclassified:
      type: object
      properties:
        voucher_id:
          type: string
        voucher_code:
          type: string
        voucher_date:
          type: string
          format: date
        direction:
          type: string
        amount:
          type: string
        reason:
          type: string
    GlIndirectCashFlow:
      type: object
      properties:
        net_profit:
          type: string
        non_cash_expenses:
          type: string
        receivable_increase:
          type: string
        inventory_increase:
          type: string
        payable_increase:
          type: string
        tax_payable_increase:
          type: string
        employee_payable_increase:
          type: string
        operating_net:
          type: string
        direct_operating_net:
          type: string
        difference:
          type: string
    GlCashFlowStatement:
      type: object
      properties:
        start_date:
          type: string
          format: date
        end_date:
          type: string
          format: date
        operating:
          $ref: '#/components/schemas/GlCashFlowSection'
        investing:
          $ref: '#/components/schemas/GlCashFlowSection'
        financing:
          $ref: '#/components/schemas/GlCashFlowSection'
        net_increase:
          type: string
        opening_cash:
          type: string
        closing_cash:
          type: string
        unclassified:
          type: array
          items:
            $ref: '#/components/schemas/GlCashFlowUnclassified'
        indirect:
          $ref: '#/components/schemas/GlIndirectCashFlow'
```

- [ ] **Step 4: 更新完整度检查**

在 `scripts/test/check-frontend.mjs` 中增加：

```js
assertFile('apps/web/src/views/finance/ReportView.vue', [
  '现金流量表',
  'cashFlow',
  'unclassified',
  'indirect'
])
assertFile('packages/sdk/src/gl.ts', ['CashFlowStatement', 'cashFlow', '/finance/reports/cash-flow'])
```

- [ ] **Step 5: 校验 YAML 和类型**

Run: `npx -y yaml@2 valid api/openapi.yaml && pnpm typecheck:web`

Expected: 成功。

- [ ] **Step 6: 提交**

```bash
git add packages/sdk/src/gl.ts api/openapi.yaml scripts/test/check-frontend.mjs
git commit -m "feat(stage-3g): update SDK and OpenAPI"
```

---

## Task 5: 前端现金流量表页签

**Files:**
- Modify: `apps/web/src/views/finance/ReportView.vue`

- [ ] **Step 1: 增加页签和展示**

在“利润表”页签后增加：

```html
<el-tab-pane label="现金流量表" name="cash">
  <div class="toolbar">
    <el-date-picker v-model="cashRange" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始日期" end-placeholder="结束日期" />
    <el-button type="primary" plain @click="loadCashFlow">查询</el-button>
  </div>
  <template v-if="cashFlow">
    <div class="summary">
      <span>期初现金：{{ cashFlow.opening_cash }}</span>
      <span>现金净增加额：{{ cashFlow.net_increase }}</span>
      <span>期末现金：{{ cashFlow.closing_cash }}</span>
    </div>

    <div class="cash-section">
      <h3>经营活动</h3>
      <el-table :data="cashFlow.operating.inflows" border>
        <el-table-column prop="name" label="流入项目" min-width="240" />
        <el-table-column prop="amount" label="金额" width="150" align="right" />
      </el-table>
      <el-table :data="cashFlow.operating.outflows" border class="detail-table">
        <el-table-column prop="name" label="流出项目" min-width="240" />
        <el-table-column prop="amount" label="金额" width="150" align="right" />
      </el-table>
      <div class="total">经营活动净额：{{ cashFlow.operating.net }}</div>
    </div>

    <div class="cash-section">
      <h3>投资活动</h3>
      <el-table :data="cashFlow.investing.inflows" border>
        <el-table-column prop="name" label="流入项目" min-width="240" />
        <el-table-column prop="amount" label="金额" width="150" align="right" />
      </el-table>
      <el-table :data="cashFlow.investing.outflows" border class="detail-table">
        <el-table-column prop="name" label="流出项目" min-width="240" />
        <el-table-column prop="amount" label="金额" width="150" align="right" />
      </el-table>
      <div class="total">投资活动净额：{{ cashFlow.investing.net }}</div>
    </div>

    <div class="cash-section">
      <h3>筹资活动</h3>
      <el-table :data="cashFlow.financing.inflows" border>
        <el-table-column prop="name" label="流入项目" min-width="240" />
        <el-table-column prop="amount" label="金额" width="150" align="right" />
      </el-table>
      <el-table :data="cashFlow.financing.outflows" border class="detail-table">
        <el-table-column prop="name" label="流出项目" min-width="240" />
        <el-table-column prop="amount" label="金额" width="150" align="right" />
      </el-table>
      <div class="total">筹资活动净额：{{ cashFlow.financing.net }}</div>
    </div>

    <h3 class="section-title">间接法调节表</h3>
    <el-table :data="indirectRows" border>
      <el-table-column prop="name" label="项目" min-width="280" />
      <el-table-column prop="amount" label="金额" width="150" align="right" />
    </el-table>
    <div v-if="cashFlow.indirect.difference !== '0.0000'" class="warning-text">
      直接法与间接法经营现金流差额：{{ cashFlow.indirect.difference }}
    </div>

    <template v-if="cashFlow.unclassified.length">
      <h3 class="section-title">未分类现金凭证</h3>
      <el-table :data="cashFlow.unclassified" border>
        <el-table-column prop="voucher_code" label="凭证号" width="160" />
        <el-table-column prop="voucher_date" label="日期" width="110" />
        <el-table-column prop="direction" label="方向" width="90" />
        <el-table-column prop="amount" label="金额" width="130" align="right" />
        <el-table-column prop="reason" label="原因" min-width="220" />
      </el-table>
    </template>
  </template>
  <el-empty v-else description="点击查询生成现金流量表" />
</el-tab-pane>
```

- [ ] **Step 2: 增加脚本状态和加载方法**

```ts
import type { CashFlowStatement } from '@mlf-erp/sdk'

const cashRange = ref<[string, string]>([firstDay, today])
const cashFlow = ref<CashFlowStatement | null>(null)

const indirectRows = computed(() => {
  const indirect = cashFlow.value?.indirect
  if (!indirect) {
    return []
  }
  return [
    { name: '净利润', amount: indirect.net_profit },
    { name: '加：非付现费用', amount: indirect.non_cash_expenses },
    { name: '减：经营性应收项目增加', amount: indirect.receivable_increase },
    { name: '减：存货增加', amount: indirect.inventory_increase },
    { name: '加：经营性应付项目增加', amount: indirect.payable_increase },
    { name: '加：应交税费增加', amount: indirect.tax_payable_increase },
    { name: '加：应付职工薪酬增加', amount: indirect.employee_payable_increase },
    { name: '经营活动产生的现金流量净额（间接法）', amount: indirect.operating_net },
    { name: '直接法经营活动现金流量净额', amount: indirect.direct_operating_net },
    { name: '差额', amount: indirect.difference }
  ]
})

async function loadCashFlow() {
  loading.value = true
  try {
    cashFlow.value = await glApi.reports.cashFlow({
      start_date: cashRange.value?.[0],
      end_date: cashRange.value?.[1]
    })
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载现金流量表失败')
  } finally {
    loading.value = false
  }
}
```

在 `onTabChange` 增加：

```ts
if (tab === 'cash') loadCashFlow()
```

- [ ] **Step 3: 增加样式**

```css
.cash-section {
  margin-top: 20px;
}

.warning-text {
  margin-top: 12px;
  color: #b45309;
  font-weight: 600;
}
```

- [ ] **Step 4: 前端检查**

Run: `pnpm typecheck:web && pnpm test:frontend && pnpm build:web`

Expected: 成功。

- [ ] **Step 5: 提交**

```bash
git add apps/web/src/views/finance/ReportView.vue
git commit -m "feat(stage-3g): add cash flow report UI"
```

---

## Task 6: Stage 3G API 测试和脚本

**Files:**
- Create: `tests/api/stage3g_test.go`
- Create: `scripts/test/stage3g-api.ps1`
- Modify: `package.json`
- Modify: `README.md`

- [ ] **Step 1: 写测试**

```go
//go:build integration

package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestStage3G(t *testing.T) {
	admin := newClient(t).login(
		env("TEST_ADMIN_USERNAME", "admin"),
		env("TEST_ADMIN_PASSWORD", "Admin@123456"),
	)

	year := ensureCashFlowYear(t, admin)
	ensureTestYear(t, admin, year)
	accounts := getArray(admin, "/finance/accounts", http.StatusOK)
	accountIDs := map[string]string{}
	for _, account := range accounts {
		accountIDs[text(account, "code")] = text(account, "id")
	}
	for _, code := range []string{"1001", "1002", "1122", "1601", "2202", "2211", "2221", "4001", "4103", "4104"} {
		if accountIDs[code] == "" {
			t.Fatalf("account %s not found", code)
		}
	}

	jan := fmt.Sprintf("%04d-01-15", year)
	janStart := fmt.Sprintf("%04d-01-01", year)
	janEnd := fmt.Sprintf("%04d-01-31", year)
	feb := fmt.Sprintf("%04d-02-15", year)
	febStart := fmt.Sprintf("%04d-02-01", year)
	febEnd := time.Date(year, time.March, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["1002"], "debit": "100.00", "credit": "0"},
		{"account_id": accountIDs["1122"], "debit": "0", "credit": "100.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["2202"], "debit": "40.00", "credit": "0"},
		{"account_id": accountIDs["1002"], "debit": "0", "credit": "40.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["2221"], "debit": "10.00", "credit": "0"},
		{"account_id": accountIDs["1002"], "debit": "0", "credit": "10.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["2211"], "debit": "20.00", "credit": "0"},
		{"account_id": accountIDs["1001"], "debit": "0", "credit": "20.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["1601"], "debit": "50.00", "credit": "0"},
		{"account_id": accountIDs["1002"], "debit": "0", "credit": "50.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["1002"], "debit": "30.00", "credit": "0"},
		{"account_id": accountIDs["1601"], "debit": "0", "credit": "30.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["1002"], "debit": "200.00", "credit": "0"},
		{"account_id": accountIDs["4001"], "debit": "0", "credit": "200.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["4104"], "debit": "15.00", "credit": "0"},
		{"account_id": accountIDs["1002"], "debit": "0", "credit": "15.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["1001"], "debit": "25.00", "credit": "0"},
		{"account_id": accountIDs["1002"], "debit": "0", "credit": "25.00"},
	})
	postCashFlowVoucher(t, admin, feb, []map[string]any{
		{"account_id": accountIDs["1002"], "debit": "5.00", "credit": "0"},
		{"account_id": accountIDs["4103"], "debit": "0", "credit": "5.00"},
	})

	report := admin.mustGet("/finance/reports/cash-flow?start_date="+janStart+"&end_date="+janEnd, http.StatusOK)
	operating, _ := report["operating"].(map[string]any)
	investing, _ := report["investing"].(map[string]any)
	financing, _ := report["financing"].(map[string]any)
	if got := number(t, cashFlowLine(t, operating, "inflows", "sales_receipts"), "amount"); !nearlyEqual(got, 100) {
		t.Fatalf("sales receipts = %v, want 100", got)
	}
	if got := number(t, cashFlowLine(t, operating, "outflows", "purchase_payments"), "amount"); !nearlyEqual(got, 40) {
		t.Fatalf("purchase payments = %v, want 40", got)
	}
	if got := number(t, cashFlowLine(t, operating, "outflows", "tax_payments"), "amount"); !nearlyEqual(got, 10) {
		t.Fatalf("tax payments = %v, want 10", got)
	}
	if got := number(t, cashFlowLine(t, operating, "outflows", "employee_payments"), "amount"); !nearlyEqual(got, 20) {
		t.Fatalf("employee payments = %v, want 20", got)
	}
	if got := number(t, cashFlowLine(t, investing, "inflows", "disposal_asset_receipts"), "amount"); !nearlyEqual(got, 30) {
		t.Fatalf("disposal receipts = %v, want 30", got)
	}
	if got := number(t, cashFlowLine(t, investing, "outflows", "asset_purchases"), "amount"); !nearlyEqual(got, 50) {
		t.Fatalf("asset purchases = %v, want 50", got)
	}
	if got := number(t, cashFlowLine(t, financing, "inflows", "capital_injections"), "amount"); !nearlyEqual(got, 200) {
		t.Fatalf("capital injections = %v, want 200", got)
	}
	if got := number(t, cashFlowLine(t, financing, "outflows", "dividend_interest_payments"), "amount"); !nearlyEqual(got, 15) {
		t.Fatalf("dividend payments = %v, want 15", got)
	}
	if got := number(t, report, "net_increase"); !nearlyEqual(got, 195) {
		t.Fatalf("net increase = %v, want 195", got)
	}
	if got := number(t, report, "opening_cash") + 195; !nearlyEqual(got, number(t, report, "closing_cash")) {
		t.Fatalf("closing cash = %v, want opening + 195", number(t, report, "closing_cash"))
	}
	indirect, _ := report["indirect"].(map[string]any)
	if got := number(t, indirect, "direct_operating_net"); !nearlyEqual(got, 30) {
		t.Fatalf("direct operating net = %v, want 30", got)
	}
	if got := number(t, indirect, "operating_net"); !nearlyEqual(got, 30) {
		t.Fatalf("indirect operating net = %v, want 30", got)
	}
	if got := number(t, indirect, "difference"); !nearlyEqual(got, 0) {
		t.Fatalf("indirect difference = %v, want 0", got)
	}
	if items, _ := report["unclassified"].([]any); len(items) != 0 {
		t.Fatalf("January unclassified count = %d, want 0", len(items))
	}

	unclassifiedReport := admin.mustGet("/finance/reports/cash-flow?start_date="+febStart+"&end_date="+febEnd, http.StatusOK)
	febOperating, _ := unclassifiedReport["operating"].(map[string]any)
	if got := number(t, cashFlowLine(t, febOperating, "inflows", "other_operating_inflows"), "amount"); !nearlyEqual(got, 5) {
		t.Fatalf("other operating inflows = %v, want 5", got)
	}
	unclassified, _ := unclassifiedReport["unclassified"].([]any)
	if len(unclassified) != 1 {
		t.Fatalf("February unclassified count = %d, want 1", len(unclassified))
	}
	if got := text(unclassified[0].(map[string]any), "amount"); got != "5.0000" {
		t.Fatalf("unclassified amount = %s, want 5.0000", got)
	}
	febIndirect, _ := unclassifiedReport["indirect"].(map[string]any)
	if got := number(t, febIndirect, "difference"); !nearlyEqual(got, -5) {
		t.Fatalf("February indirect difference = %v, want -5", got)
	}

	unauthorized := newClient(t)
	unauthorized.expectError(http.MethodGet, "/finance/reports/cash-flow", nil, http.StatusUnauthorized, "UNAUTHORIZED")
	admin.expectError(http.MethodGet, "/finance/reports/cash-flow?start_date=bad-date", nil, http.StatusBadRequest, "BAD_REQUEST")
}

func ensureCashFlowYear(t *testing.T, admin *client) int {
	t.Helper()
	for year := 2080; year <= 2500; year++ {
		if len(getArray(admin, periodsPath(year), http.StatusOK)) == 0 {
			return year
		}
	}
	t.Fatal("no clean year left for cash flow test")
	return 0
}

func postCashFlowVoucher(t *testing.T, admin *client, date string, lines []map[string]any) string {
	t.Helper()
	voucher := admin.mustPost("/finance/vouchers", map[string]any{
		"voucher_date": date,
		"memo":         "stage3g cash flow",
		"lines":        lines,
	}, http.StatusCreated)
	voucherID := text(voucher, "id")
	admin.mustPost("/finance/vouchers/"+voucherID+"/post", nil, http.StatusOK)
	return voucherID
}

func cashFlowLine(t *testing.T, section map[string]any, direction, code string) map[string]any {
	t.Helper()
	items, _ := section[direction].([]any)
	for _, raw := range items {
		line := raw.(map[string]any)
		if text(line, "code") == code {
			return line
		}
	}
	t.Fatalf("cash flow line %s/%s not found", direction, code)
	return nil
}
```

测试使用干净年份和手工凭证，避免依赖库存和 Demo 业务数据。核心断言：

- 销售收款 `Dr 1002 / Cr 1122` → `sales_receipts` 100。
- 采购付款 `Dr 2202 / Cr 1002` → `purchase_payments` 40。
- 税费 `Dr 2221 / Cr 1002` → `tax_payments` 10。
- 薪酬 `Dr 2211 / Cr 1001` → `employee_payments` 20。
- 资产购买 `Dr 1601 / Cr 1002` 50；处置 `Dr 1002 / Cr 1601` 30。
- 投资 `Dr 1002 / Cr 4001` 200；分配利润 `Dr 4104 / Cr 1002` 15。
- 现金/银行互转 `Dr 1001 / Cr 1002` 25 不计入现金流。
- 未分类 `Dr 1002 / Cr 4103` 5 → 其他经营流入，返回未分类明细。
- 直接法经营净额 30；投资净额 -20；筹资净额 185；净增加额 195。
- 期初现金 + 净增加额 = 期末现金。
- 间接法经营净额 30，与直接法差额 0；未分类月份差额 -5。
- 401、非法日期。


- [ ] **Step 2: 增加脚本和 package**

复制 `scripts/test/stage3f-api.ps1` 为 `stage3g-api.ps1`，替换 `stage3f`/`TestStage3F`/`Stage 3F` 为 `stage3g`/`TestStage3G`/`Stage 3G`。

`package.json` 增加：

```json
"test:api:stage3g": "powershell -ExecutionPolicy Bypass -File ./scripts/test/stage3g-api.ps1",
```

- [ ] **Step 3: 运行测试**

Run: `pnpm test:api:stage3g`

Expected: PASS。

- [ ] **Step 4: 运行 Stage 3 全链路**

Run:

```bash
go test -tags=integration ./tests/api -run 'TestStage3' -count=1 -v
```

Expected: Stage 3B–3G 全部通过。

- [ ] **Step 5: 提交**

```bash
git add tests/api/stage3g_test.go scripts/test/stage3g-api.ps1 package.json README.md
git commit -m "test(stage-3g): add cash flow API tests"
```

---

## Task 7: 文档、静态检查、汇总报告和推送

**Files:**
- Modify: `README.md`
- Modify: `internal/modules/gl/README.md`
- Create: `docs/reports/2026-11-18-stage-3g-cash-flow.md`

- [ ] **Step 1: 更新 README 和 GL 模块文档**

补充：

- 当前状态增加 Stage 3G。
- 测试命令 `pnpm test:api:stage3g`。
- 现金流项目、直接法、间接法、未分类、已知限制。
- GL API 表增加 `/finance/reports/cash-flow`。

- [ ] **Step 2: 汇总报告**

报告包含：

- 完成范围。
- 标准项目和科目映射。
- 直接法算法和间接法公式。
- API/SDK/前端。
- 测试结果。
- 代码审核章节（先占位，审核后补）。
- 已知限制。
- 用户审阅清单。

- [ ] **Step 3: 静态检查**

```bash
gofmt -w ./cmd ./internal ./db ./tests
gofmt -l ./cmd ./internal ./db ./tests
go test ./...
go vet ./...
pnpm typecheck:web
pnpm test:frontend
pnpm build:web
npx -y yaml@2 valid api/openapi.yaml
```

Expected: 全部通过。

- [ ] **Step 4: 提交文档和报告草稿**

```bash
git add -A
git commit -m "docs(stage-3g): add cash flow report draft"
```

---

## Task 8: 独立代码审核、修复和推送

**Files:**
- Modify: `docs/reports/2026-11-18-stage-3g-cash-flow.md`

- [ ] **Step 1: 派发独立代码审核**

审核范围：Stage 3G 实现提交。重点：

- 现金流分摊算法和尾差处理。
- 现金/银行互转识别。
- 科目映射和未分类明细。
- 直接法/间接法勾稽。
- 期初/期末现金和余额查询范围。
- API 权限、日期校验、数据范围。
- 前端展示和 SDK/OpenAPI 一致性。
- 测试覆盖。

- [ ] **Step 2: 逐条核验 Critical/Important**

- 对每个问题先验证代码和实际行为。
- Critical 必须修复并补回归测试。
- Important 根据影响修复或记录限制。
- Minor 记录在报告。

- [ ] **Step 3: 回归验证**

```bash
pnpm test:api:stage3g
go test -tags=integration ./tests/api -run 'TestStage3' -count=1 -v
gofmt -l ./cmd ./internal ./db ./tests
go test ./...
go vet ./...
pnpm typecheck:web
pnpm test:frontend
pnpm build:web
npx -y yaml@2 valid api/openapi.yaml
```

Expected: 全部通过。

- [ ] **Step 4: 填写审核章节并提交**

在报告中补充：

- 审核方式、范围、结论。
- 已修复问题。
- 未修复问题和已知限制。
- 最终提交哈希。

```bash
git add -A
git commit -m "fix(stage-3g): address code review findings and finalize report"
git push -u origin stage-3g-cash-flow
```

---

## 验收清单

- [ ] 直接法现金流量表主表可用。
- [ ] 经营、投资、筹资项目归类正确。
- [ ] 现金/银行互转不进入现金流。
- [ ] 未分类金额进入其他经营并返回明细。
- [ ] 期初/期末现金与净增加额勾稽。
- [ ] 间接法调节表可用并显示差额。
- [ ] SDK/OpenAPI/前端完整。
- [ ] Stage 3G API 测试、Stage 3 回归、静态检查和构建通过。
- [ ] 代码审核和汇总报告完成。
