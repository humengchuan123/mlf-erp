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
