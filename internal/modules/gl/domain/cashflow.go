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

var cashFlowItems = []CashFlowItemDef{
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
	items := make(map[string]CashFlowItemDef, len(cashFlowItems))
	for _, item := range cashFlowItems {
		items[item.Code] = item
	}
	return items
}()

// cashFlowClassifierItemCodes lists every catalog item code produced by
// ClassifyCashFlow so catalog drift fails fast during init.
var cashFlowClassifierItemCodes = []string{
	"sales_receipts",
	"tax_refunds",
	"other_operating_inflows",
	"purchase_payments",
	"employee_payments",
	"tax_payments",
	"other_operating_outflows",
	"disposal_asset_receipts",
	"other_investing_inflows",
	"asset_purchases",
	"investment_payments",
	"other_investing_outflows",
	"capital_injections",
	"borrowings",
	"other_financing_inflows",
	"debt_repayments",
	"dividend_interest_payments",
	"other_financing_outflows",
}

func init() {
	seen := make(map[string]bool, len(cashFlowItems))
	for _, item := range cashFlowItems {
		if item.Code == "" {
			panic("cash flow catalog: item code must not be empty")
		}
		if seen[item.Code] {
			panic("cash flow catalog: duplicate item code " + item.Code)
		}
		seen[item.Code] = true
		switch item.Category {
		case CashFlowOperating, CashFlowInvesting, CashFlowFinancing:
		default:
			panic("cash flow catalog: invalid category for " + item.Code + ": " + item.Category)
		}
		switch item.Direction {
		case CashFlowInflow, CashFlowOutflow:
		default:
			panic("cash flow catalog: invalid direction for " + item.Code + ": " + item.Direction)
		}
	}
	for _, code := range cashFlowClassifierItemCodes {
		mustCashFlowItem(code)
	}
}

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
	NetProfit               decimal.Decimal
	NonCashExpenses         decimal.Decimal
	ReceivableIncrease      decimal.Decimal
	InventoryIncrease       decimal.Decimal
	PayableIncrease         decimal.Decimal
	TaxPayableIncrease      decimal.Decimal
	EmployeePayableIncrease decimal.Decimal
	OperatingNet            decimal.Decimal
	DirectOperatingNet      decimal.Decimal
	Difference              decimal.Decimal
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

// CashFlowItemDefs returns a copy of the standard cash flow item catalog.
func CashFlowItemDefs() []CashFlowItemDef {
	items := make([]CashFlowItemDef, len(cashFlowItems))
	copy(items, cashFlowItems)
	return items
}

func CashFlowItem(code string) (CashFlowItemDef, bool) {
	item, ok := cashFlowItemMap[code]
	return item, ok
}

func mustCashFlowItem(code string) CashFlowItemDef {
	item, ok := cashFlowItemMap[code]
	if !ok {
		panic("cash flow catalog: unknown item code " + code)
	}
	return item
}

// CashFlowAccountRoot returns up to the first four leading digits of an
// account code, so subaccounts and dotted codes map to their standard root.
func CashFlowAccountRoot(code string) string {
	code = strings.TrimSpace(code)
	digits := make([]byte, 0, 4)
	for i := 0; i < len(code) && len(digits) < 4; i++ {
		if code[i] < '0' || code[i] > '9' {
			break
		}
		digits = append(digits, code[i])
	}
	return string(digits)
}

func ClassifyCashFlow(accountCode string, inflow bool) (CashFlowItemDef, bool) {
	code := CashFlowAccountRoot(accountCode)
	fallback := "other_operating_inflows"
	if !inflow {
		fallback = "other_operating_outflows"
	}
	switch {
	case code == "1101":
		if inflow {
			return mustCashFlowItem("other_investing_inflows"), true
		}
		return mustCashFlowItem("other_investing_outflows"), true
	case code == "1122":
		if inflow {
			return mustCashFlowItem("sales_receipts"), true
		}
		return mustCashFlowItem("other_operating_outflows"), true
	case code == "2202" || code == "1405":
		if inflow {
			return mustCashFlowItem("other_operating_inflows"), true
		}
		return mustCashFlowItem("purchase_payments"), true
	case code == "2211":
		if inflow {
			return mustCashFlowItem("other_operating_inflows"), true
		}
		return mustCashFlowItem("employee_payments"), true
	case strings.HasPrefix(code, "2221"):
		if inflow {
			return mustCashFlowItem("tax_refunds"), true
		}
		return mustCashFlowItem("tax_payments"), true
	case code == "1501" || code == "1511":
		if inflow {
			return mustCashFlowItem("other_investing_inflows"), true
		}
		return mustCashFlowItem("investment_payments"), true
	case strings.HasPrefix(code, "16") || strings.HasPrefix(code, "17"):
		if inflow {
			return mustCashFlowItem("disposal_asset_receipts"), true
		}
		return mustCashFlowItem("asset_purchases"), true
	case strings.HasPrefix(code, "2001") || strings.HasPrefix(code, "2501"):
		if inflow {
			return mustCashFlowItem("borrowings"), true
		}
		return mustCashFlowItem("debt_repayments"), true
	case strings.HasPrefix(code, "4001"):
		if inflow {
			return mustCashFlowItem("capital_injections"), true
		}
		return mustCashFlowItem("other_financing_outflows"), true
	case strings.HasPrefix(code, "4104"):
		if inflow {
			return mustCashFlowItem("other_financing_inflows"), true
		}
		return mustCashFlowItem("dividend_interest_payments"), true
	case strings.HasPrefix(code, "6001") || strings.HasPrefix(code, "6051"):
		if inflow {
			return mustCashFlowItem("sales_receipts"), true
		}
		return mustCashFlowItem("other_operating_outflows"), true
	case strings.HasPrefix(code, "6401") || strings.HasPrefix(code, "6601") ||
		strings.HasPrefix(code, "6602") || strings.HasPrefix(code, "6603") ||
		strings.HasPrefix(code, "6801"):
		if inflow {
			return mustCashFlowItem("other_operating_inflows"), true
		}
		return mustCashFlowItem("other_operating_outflows"), true
	default:
		return mustCashFlowItem(fallback), false
	}
}

func AllocateCashFlow(voucher CashFlowVoucher, cashCodes map[string]bool) []CashFlowAllocation {
	cashRoots := make(map[string]bool, len(cashCodes))
	for code := range cashCodes {
		if root := CashFlowAccountRoot(code); root != "" {
			cashRoots[root] = true
		}
	}

	cashIn := decimal.Zero
	cashOut := decimal.Zero
	creditTotal := decimal.Zero
	debitTotal := decimal.Zero
	credits := make([]CashFlowVoucherLine, 0)
	debits := make([]CashFlowVoucherLine, 0)
	for _, line := range voucher.Lines {
		if cashRoots[CashFlowAccountRoot(line.AccountCode)] {
			cashIn = cashIn.Add(line.Debit)
			cashOut = cashOut.Add(line.Credit)
			continue
		}
		if line.Credit.IsPositive() {
			credits = append(credits, line)
			creditTotal = creditTotal.Add(line.Credit)
		}
		if line.Debit.IsPositive() {
			debits = append(debits, line)
			debitTotal = debitTotal.Add(line.Debit)
		}
	}

	businessIn := decimal.Min(cashIn, creditTotal)
	businessOut := decimal.Min(cashOut, debitTotal)
	unmatchedIn := cashIn.Sub(businessIn)
	unmatchedOut := cashOut.Sub(businessOut)
	transfer := decimal.Min(unmatchedIn, unmatchedOut)
	unmatchedIn = unmatchedIn.Sub(transfer)
	unmatchedOut = unmatchedOut.Sub(transfer)

	allocations := make([]CashFlowAllocation, 0)
	if businessIn.IsPositive() {
		allocations = append(allocations, allocateCashFlowSide(voucher, businessIn, credits, true)...)
	}
	if businessOut.IsPositive() {
		allocations = append(allocations, allocateCashFlowSide(voucher, businessOut, debits, false)...)
	}
	if unmatchedIn.IsPositive() {
		allocations = append(allocations, unclassifiedCashFlowAllocation(voucher, "other_operating_inflows", unmatchedIn))
	}
	if unmatchedOut.IsPositive() {
		allocations = append(allocations, unclassifiedCashFlowAllocation(voucher, "other_operating_outflows", unmatchedOut))
	}
	return allocations
}

func allocateCashFlowSide(voucher CashFlowVoucher, amount decimal.Decimal, counterparts []CashFlowVoucherLine, inflow bool) []CashFlowAllocation {
	total := decimal.Zero
	for _, line := range counterparts {
		weight := line.Credit
		if !inflow {
			weight = line.Debit
		}
		total = total.Add(weight)
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
		allocated := amount.Mul(weight).Div(total).Truncate(4)
		if i == len(counterparts)-1 {
			allocated = remaining.Round(4)
		}
		remaining = remaining.Sub(allocated)
		item, classified := ClassifyCashFlow(line.AccountCode, inflow)
		reason := ""
		if !classified {
			reason = "未识别科目 " + strings.TrimSpace(line.AccountCode)
		}
		allocations = append(allocations, CashFlowAllocation{
			VoucherID: voucher.VoucherID, VoucherCode: voucher.VoucherCode, VoucherDate: voucher.VoucherDate,
			ItemCode: item.Code, Category: item.Category, Direction: item.Direction,
			Amount: allocated, Unclassified: !classified, Reason: reason,
		})
	}
	return allocations
}

func unclassifiedCashFlowAllocation(voucher CashFlowVoucher, itemCode string, amount decimal.Decimal) CashFlowAllocation {
	item := mustCashFlowItem(itemCode)
	return CashFlowAllocation{
		VoucherID: voucher.VoucherID, VoucherCode: voucher.VoucherCode, VoucherDate: voucher.VoucherDate,
		ItemCode: item.Code, Category: item.Category, Direction: item.Direction,
		Amount: amount.Round(4), Unclassified: true, Reason: "无对方科目",
	}
}
