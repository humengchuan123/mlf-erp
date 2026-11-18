package domain

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

var cashFlowTestDate = time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

func TestCashFlowAccountRoot(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{"plain code", "1122", "1122"},
		{"numeric subaccount", "112201", "1122"},
		{"dotted subaccount", "1122.01", "1122"},
		{"cash dotted subaccount", "1002.01", "1002"},
		{"surrounding whitespace", " 2202 ", "2202"},
		{"short code", "123", "123"},
		{"leading non-digit", "AB1122", ""},
		{"empty code", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CashFlowAccountRoot(tt.code); got != tt.want {
				t.Fatalf("CashFlowAccountRoot(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestCashFlowCatalog(t *testing.T) {
	items := CashFlowItemDefs()
	if len(items) != 18 {
		t.Fatalf("catalog size = %d, want 18", len(items))
	}
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if item.Code == "" {
			t.Fatal("catalog item has empty code")
		}
		if item.Name == "" {
			t.Fatalf("catalog item %s has empty name", item.Code)
		}
		if seen[item.Code] {
			t.Fatalf("duplicate catalog item code %s", item.Code)
		}
		seen[item.Code] = true
		switch item.Category {
		case CashFlowOperating, CashFlowInvesting, CashFlowFinancing:
		default:
			t.Fatalf("catalog item %s has invalid category %q", item.Code, item.Category)
		}
		switch item.Direction {
		case CashFlowInflow, CashFlowOutflow:
		default:
			t.Fatalf("catalog item %s has invalid direction %q", item.Code, item.Direction)
		}
		got, ok := CashFlowItem(item.Code)
		if !ok {
			t.Fatalf("CashFlowItem(%s) not found", item.Code)
		}
		if got != item {
			t.Fatalf("CashFlowItem(%s) = %+v, want %+v", item.Code, got, item)
		}
	}
	if _, ok := CashFlowItem("not_a_real_item"); ok {
		t.Fatal("CashFlowItem accepted an unknown code")
	}
	items[0].Name = "mutated"
	if CashFlowItemDefs()[0].Name == "mutated" {
		t.Fatal("CashFlowItemDefs must return a copy")
	}
}

func TestCashFlowEveryItemReachable(t *testing.T) {
	samples := []struct {
		code   string
		inflow bool
	}{
		{"1122", true},
		{"1122", false},
		{"2202", true},
		{"2202", false},
		{"2211", false},
		{"2221", true},
		{"2221", false},
		{"1601", true},
		{"1601", false},
		{"1101", true},
		{"1101", false},
		{"1501", true},
		{"1501", false},
		{"2001", true},
		{"2001", false},
		{"4001", true},
		{"4001", false},
		{"4104", true},
		{"4104", false},
	}
	reached := make(map[string]bool)
	for _, sample := range samples {
		item, _ := ClassifyCashFlow(sample.code, sample.inflow)
		reached[item.Code] = true
	}
	for _, item := range CashFlowItemDefs() {
		if !reached[item.Code] {
			t.Fatalf("cash flow item %s is not reachable via ClassifyCashFlow", item.Code)
		}
	}
}

func TestClassifyCashFlow(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		inflow         bool
		wantCode       string
		wantCategory   string
		wantDirection  string
		wantClassified bool
	}{
		{"1122 inflow", "1122", true, "sales_receipts", CashFlowOperating, CashFlowInflow, true},
		{"1122 outflow", "1122", false, "other_operating_outflows", CashFlowOperating, CashFlowOutflow, true},
		{"2202 inflow", "2202", true, "other_operating_inflows", CashFlowOperating, CashFlowInflow, true},
		{"2202 outflow", "2202", false, "purchase_payments", CashFlowOperating, CashFlowOutflow, true},
		{"1405 inflow", "1405", true, "other_operating_inflows", CashFlowOperating, CashFlowInflow, true},
		{"1405 outflow", "1405", false, "purchase_payments", CashFlowOperating, CashFlowOutflow, true},
		{"2211 inflow", "2211", true, "other_operating_inflows", CashFlowOperating, CashFlowInflow, true},
		{"2211 outflow", "2211", false, "employee_payments", CashFlowOperating, CashFlowOutflow, true},
		{"2221 inflow", "2221", true, "tax_refunds", CashFlowOperating, CashFlowInflow, true},
		{"2221 outflow", "2221", false, "tax_payments", CashFlowOperating, CashFlowOutflow, true},
		{"6001 inflow", "6001", true, "sales_receipts", CashFlowOperating, CashFlowInflow, true},
		{"6001 outflow", "6001", false, "other_operating_outflows", CashFlowOperating, CashFlowOutflow, true},
		{"6051 inflow", "6051", true, "sales_receipts", CashFlowOperating, CashFlowInflow, true},
		{"6051 outflow", "6051", false, "other_operating_outflows", CashFlowOperating, CashFlowOutflow, true},
		{"6401 inflow", "6401", true, "other_operating_inflows", CashFlowOperating, CashFlowInflow, true},
		{"6401 outflow", "6401", false, "other_operating_outflows", CashFlowOperating, CashFlowOutflow, true},
		{"6601 inflow", "6601", true, "other_operating_inflows", CashFlowOperating, CashFlowInflow, true},
		{"6601 outflow", "6601", false, "other_operating_outflows", CashFlowOperating, CashFlowOutflow, true},
		{"6602 inflow", "6602", true, "other_operating_inflows", CashFlowOperating, CashFlowInflow, true},
		{"6602 outflow", "6602", false, "other_operating_outflows", CashFlowOperating, CashFlowOutflow, true},
		{"6603 inflow", "6603", true, "other_operating_inflows", CashFlowOperating, CashFlowInflow, true},
		{"6603 outflow", "6603", false, "other_operating_outflows", CashFlowOperating, CashFlowOutflow, true},
		{"6801 inflow", "6801", true, "other_operating_inflows", CashFlowOperating, CashFlowInflow, true},
		{"6801 outflow", "6801", false, "other_operating_outflows", CashFlowOperating, CashFlowOutflow, true},
		{"1601 inflow", "1601", true, "disposal_asset_receipts", CashFlowInvesting, CashFlowInflow, true},
		{"1601 outflow", "1601", false, "asset_purchases", CashFlowInvesting, CashFlowOutflow, true},
		{"1701 inflow", "1701", true, "disposal_asset_receipts", CashFlowInvesting, CashFlowInflow, true},
		{"1701 outflow", "1701", false, "asset_purchases", CashFlowInvesting, CashFlowOutflow, true},
		{"2001 inflow", "2001", true, "borrowings", CashFlowFinancing, CashFlowInflow, true},
		{"2001 outflow", "2001", false, "debt_repayments", CashFlowFinancing, CashFlowOutflow, true},
		{"2501 inflow", "2501", true, "borrowings", CashFlowFinancing, CashFlowInflow, true},
		{"2501 outflow", "2501", false, "debt_repayments", CashFlowFinancing, CashFlowOutflow, true},
		{"4001 inflow", "4001", true, "capital_injections", CashFlowFinancing, CashFlowInflow, true},
		{"4001 outflow", "4001", false, "other_financing_outflows", CashFlowFinancing, CashFlowOutflow, true},
		{"4104 inflow", "4104", true, "other_financing_inflows", CashFlowFinancing, CashFlowInflow, true},
		{"4104 outflow", "4104", false, "dividend_interest_payments", CashFlowFinancing, CashFlowOutflow, true},
		{"1101 inflow", "1101", true, "other_investing_inflows", CashFlowInvesting, CashFlowInflow, true},
		{"1101 outflow", "1101", false, "other_investing_outflows", CashFlowInvesting, CashFlowOutflow, true},
		{"1501 inflow", "1501", true, "other_investing_inflows", CashFlowInvesting, CashFlowInflow, true},
		{"1501 outflow", "1501", false, "investment_payments", CashFlowInvesting, CashFlowOutflow, true},
		{"1511 inflow", "1511", true, "other_investing_inflows", CashFlowInvesting, CashFlowInflow, true},
		{"1511 outflow", "1511", false, "investment_payments", CashFlowInvesting, CashFlowOutflow, true},
		{"unknown inflow", "9999", true, "other_operating_inflows", CashFlowOperating, CashFlowInflow, false},
		{"unknown outflow", "9999", false, "other_operating_outflows", CashFlowOperating, CashFlowOutflow, false},
		{"1122 subaccount inflow", "112201", true, "sales_receipts", CashFlowOperating, CashFlowInflow, true},
		{"1122 subaccount outflow", "112201", false, "other_operating_outflows", CashFlowOperating, CashFlowOutflow, true},
		{"2221 subaccount inflow", "222101", true, "tax_refunds", CashFlowOperating, CashFlowInflow, true},
		{"2221 subaccount outflow", "222101", false, "tax_payments", CashFlowOperating, CashFlowOutflow, true},
		{"dotted subaccount inflow", "1122.01", true, "sales_receipts", CashFlowOperating, CashFlowInflow, true},
		{"whitespace inflow", " 1122 ", true, "sales_receipts", CashFlowOperating, CashFlowInflow, true},
		{"whitespace outflow", " 2202 ", false, "purchase_payments", CashFlowOperating, CashFlowOutflow, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, classified := ClassifyCashFlow(tt.code, tt.inflow)
			if item.Code != tt.wantCode {
				t.Fatalf("item code = %s, want %s", item.Code, tt.wantCode)
			}
			if item.Category != tt.wantCategory {
				t.Fatalf("category = %s, want %s", item.Category, tt.wantCategory)
			}
			if item.Direction != tt.wantDirection {
				t.Fatalf("direction = %s, want %s", item.Direction, tt.wantDirection)
			}
			if classified != tt.wantClassified {
				t.Fatalf("classified = %v, want %v", classified, tt.wantClassified)
			}
		})
	}
}

func TestAllocateCashFlowSingleCounterpart(t *testing.T) {
	voucher := CashFlowVoucher{
		VoucherID: "v1", VoucherCode: "GL-1", VoucherDate: cashFlowTestDate,
		Lines: []CashFlowVoucherLine{
			{AccountCode: "1002", Debit: decimal.NewFromInt(100)},
			{AccountCode: "1122", Credit: decimal.NewFromInt(100)},
		},
	}
	allocations := AllocateCashFlow(voucher, map[string]bool{"1001": true, "1002": true})
	if len(allocations) != 1 {
		t.Fatalf("allocations = %d, want 1", len(allocations))
	}
	allocation := allocations[0]
	assertCashFlowAllocationSource(t, allocation, voucher)
	if allocation.ItemCode != "sales_receipts" {
		t.Fatalf("item = %s, want sales_receipts", allocation.ItemCode)
	}
	if allocation.Category != CashFlowOperating {
		t.Fatalf("category = %s, want %s", allocation.Category, CashFlowOperating)
	}
	if allocation.Direction != CashFlowInflow {
		t.Fatalf("direction = %s, want %s", allocation.Direction, CashFlowInflow)
	}
	if !allocation.Amount.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("amount = %s, want 100", allocation.Amount)
	}
	if allocation.Unclassified {
		t.Fatal("allocation must be classified")
	}
	if allocation.Reason != "" {
		t.Fatalf("reason = %q, want empty", allocation.Reason)
	}
}

func TestAllocateCashFlowSingleCounterpartOutflow(t *testing.T) {
	voucher := CashFlowVoucher{
		VoucherID: "v-out", VoucherCode: "GL-OUT", VoucherDate: cashFlowTestDate,
		Lines: []CashFlowVoucherLine{
			{AccountCode: "2202", Debit: decimal.NewFromInt(20)},
			{AccountCode: "1002", Credit: decimal.NewFromInt(20)},
		},
	}
	allocations := AllocateCashFlow(voucher, map[string]bool{"1002": true})
	if len(allocations) != 1 {
		t.Fatalf("allocations = %d, want 1", len(allocations))
	}
	allocation := allocations[0]
	assertCashFlowAllocationSource(t, allocation, voucher)
	if allocation.ItemCode != "purchase_payments" {
		t.Fatalf("item = %s, want purchase_payments", allocation.ItemCode)
	}
	if allocation.Category != CashFlowOperating {
		t.Fatalf("category = %s, want %s", allocation.Category, CashFlowOperating)
	}
	if allocation.Direction != CashFlowOutflow {
		t.Fatalf("direction = %s, want %s", allocation.Direction, CashFlowOutflow)
	}
	if !allocation.Amount.Equal(decimal.NewFromInt(20)) {
		t.Fatalf("amount = %s, want 20", allocation.Amount)
	}
	if allocation.Unclassified {
		t.Fatal("allocation must be classified")
	}
}

func TestAllocateCashFlowMultipleCounterpartsAndRounding(t *testing.T) {
	voucher := CashFlowVoucher{
		VoucherID: "v2", VoucherCode: "GL-2", VoucherDate: cashFlowTestDate,
		Lines: []CashFlowVoucherLine{
			{AccountCode: "1002", Debit: decimal.RequireFromString("10")},
			{AccountCode: "1122", Credit: decimal.RequireFromString("10")},
			{AccountCode: "2202", Credit: decimal.RequireFromString("20")},
		},
	}
	allocations := AllocateCashFlow(voucher, map[string]bool{"1002": true})
	if len(allocations) != 2 {
		t.Fatalf("allocations = %d, want 2", len(allocations))
	}
	for _, allocation := range allocations {
		assertCashFlowAllocationSource(t, allocation, voucher)
	}
	total := decimal.Zero
	for _, allocation := range allocations {
		total = total.Add(allocation.Amount)
	}
	if !total.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("allocated total = %s, want 10", total)
	}
	if allocations[0].ItemCode != "sales_receipts" {
		t.Fatalf("first item = %s, want sales_receipts", allocations[0].ItemCode)
	}
	if !allocations[0].Amount.Equal(decimal.RequireFromString("3.3333")) {
		t.Fatalf("first amount = %s, want 3.3333", allocations[0].Amount)
	}
	if allocations[0].Direction != CashFlowInflow {
		t.Fatalf("first direction = %s, want %s", allocations[0].Direction, CashFlowInflow)
	}
	if allocations[1].ItemCode != "other_operating_inflows" {
		t.Fatalf("second item = %s, want other_operating_inflows", allocations[1].ItemCode)
	}
	if !allocations[1].Amount.Equal(decimal.RequireFromString("6.6667")) {
		t.Fatalf("second amount = %s, want 6.6667", allocations[1].Amount)
	}
	if allocations[0].Unclassified || allocations[1].Unclassified {
		t.Fatal("allocations must be classified")
	}
}

func TestAllocateCashFlowMultipleOutflowCounterpartsAndRounding(t *testing.T) {
	voucher := CashFlowVoucher{
		VoucherID: "v2-out", VoucherCode: "GL-2-OUT", VoucherDate: cashFlowTestDate,
		Lines: []CashFlowVoucherLine{
			{AccountCode: "2202", Debit: decimal.RequireFromString("10")},
			{AccountCode: "1122", Debit: decimal.RequireFromString("20")},
			{AccountCode: "1002", Credit: decimal.RequireFromString("10")},
		},
	}
	allocations := AllocateCashFlow(voucher, map[string]bool{"1002": true})
	if len(allocations) != 2 {
		t.Fatalf("allocations = %d, want 2", len(allocations))
	}
	for _, allocation := range allocations {
		assertCashFlowAllocationSource(t, allocation, voucher)
	}
	total := decimal.Zero
	for _, allocation := range allocations {
		total = total.Add(allocation.Amount)
	}
	if !total.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("allocated total = %s, want 10", total)
	}
	if allocations[0].ItemCode != "purchase_payments" {
		t.Fatalf("first item = %s, want purchase_payments", allocations[0].ItemCode)
	}
	if !allocations[0].Amount.Equal(decimal.RequireFromString("3.3333")) {
		t.Fatalf("first amount = %s, want 3.3333", allocations[0].Amount)
	}
	if allocations[0].Direction != CashFlowOutflow {
		t.Fatalf("first direction = %s, want %s", allocations[0].Direction, CashFlowOutflow)
	}
	if allocations[1].ItemCode != "other_operating_outflows" {
		t.Fatalf("second item = %s, want other_operating_outflows", allocations[1].ItemCode)
	}
	if !allocations[1].Amount.Equal(decimal.RequireFromString("6.6667")) {
		t.Fatalf("second amount = %s, want 6.6667", allocations[1].Amount)
	}
	if allocations[0].Unclassified || allocations[1].Unclassified {
		t.Fatal("allocations must be classified")
	}
}

func TestAllocateCashFlowIgnoresInternalTransfer(t *testing.T) {
	voucher := CashFlowVoucher{
		VoucherID: "v3", VoucherCode: "GL-3", VoucherDate: cashFlowTestDate,
		Lines: []CashFlowVoucherLine{
			{AccountCode: "1001", Debit: decimal.NewFromInt(25)},
			{AccountCode: "1002", Credit: decimal.NewFromInt(25)},
		},
	}
	if got := AllocateCashFlow(voucher, map[string]bool{"1001": true, "1002": true}); len(got) != 0 {
		t.Fatalf("internal transfer allocations = %d, want 0", len(got))
	}
}

func TestAllocateCashFlowWithoutCashMovement(t *testing.T) {
	voucher := CashFlowVoucher{
		VoucherID: "v-no-cash", VoucherCode: "GL-NO-CASH", VoucherDate: cashFlowTestDate,
		Lines: []CashFlowVoucherLine{
			{AccountCode: "1122", Debit: decimal.NewFromInt(10)},
			{AccountCode: "2202", Credit: decimal.NewFromInt(10)},
		},
	}
	if got := AllocateCashFlow(voucher, map[string]bool{"1002": true}); len(got) != 0 {
		t.Fatalf("non-cash allocations = %d, want 0", len(got))
	}
}

func TestAllocateCashFlowMixedTransferAndBusiness(t *testing.T) {
	tests := []struct {
		name          string
		lines         []CashFlowVoucherLine
		wantItem      string
		wantDirection string
		wantAmount    string
	}{
		{
			name: "sales receipt with transfer",
			lines: []CashFlowVoucherLine{
				{AccountCode: "1002", Debit: decimal.NewFromInt(100)},
				{AccountCode: "1001", Debit: decimal.NewFromInt(40)},
				{AccountCode: "1122", Credit: decimal.NewFromInt(100)},
				{AccountCode: "1002", Credit: decimal.NewFromInt(40)},
			},
			wantItem:      "sales_receipts",
			wantDirection: CashFlowInflow,
			wantAmount:    "100",
		},
		{
			name: "purchase payment with transfer",
			lines: []CashFlowVoucherLine{
				{AccountCode: "1001", Debit: decimal.NewFromInt(30)},
				{AccountCode: "1002", Credit: decimal.NewFromInt(30)},
				{AccountCode: "2202", Debit: decimal.NewFromInt(20)},
				{AccountCode: "1002", Credit: decimal.NewFromInt(20)},
			},
			wantItem:      "purchase_payments",
			wantDirection: CashFlowOutflow,
			wantAmount:    "20",
		},
		{
			name: "bank fee with transfer",
			lines: []CashFlowVoucherLine{
				{AccountCode: "1001", Debit: decimal.NewFromInt(100)},
				{AccountCode: "1002", Credit: decimal.NewFromInt(100)},
				{AccountCode: "6603", Debit: decimal.NewFromInt(5)},
				{AccountCode: "1002", Credit: decimal.NewFromInt(5)},
			},
			wantItem:      "other_operating_outflows",
			wantDirection: CashFlowOutflow,
			wantAmount:    "5",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			voucher := CashFlowVoucher{
				VoucherID: "v-mixed", VoucherCode: "GL-MIXED", VoucherDate: cashFlowTestDate,
				Lines: tt.lines,
			}
			allocations := AllocateCashFlow(voucher, map[string]bool{"1001": true, "1002": true})
			if len(allocations) != 1 {
				t.Fatalf("allocations = %d, want 1: %+v", len(allocations), allocations)
			}
			allocation := allocations[0]
			assertCashFlowAllocationSource(t, allocation, voucher)
			if allocation.ItemCode != tt.wantItem {
				t.Fatalf("item = %s, want %s", allocation.ItemCode, tt.wantItem)
			}
			if allocation.Direction != tt.wantDirection {
				t.Fatalf("direction = %s, want %s", allocation.Direction, tt.wantDirection)
			}
			if !allocation.Amount.Equal(decimal.RequireFromString(tt.wantAmount)) {
				t.Fatalf("amount = %s, want %s", allocation.Amount, tt.wantAmount)
			}
			if allocation.Unclassified {
				t.Fatalf("allocation must be classified: %+v", allocation)
			}
		})
	}
}

func TestAllocateCashFlowNoCounterpart(t *testing.T) {
	tests := []struct {
		name          string
		line          CashFlowVoucherLine
		wantItem      string
		wantDirection string
	}{
		{
			name:          "inflow without counterpart",
			line:          CashFlowVoucherLine{AccountCode: "1002", Debit: decimal.NewFromInt(15)},
			wantItem:      "other_operating_inflows",
			wantDirection: CashFlowInflow,
		},
		{
			name:          "outflow without counterpart",
			line:          CashFlowVoucherLine{AccountCode: "1002", Credit: decimal.NewFromInt(15)},
			wantItem:      "other_operating_outflows",
			wantDirection: CashFlowOutflow,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			voucher := CashFlowVoucher{
				VoucherID: "v-unmatched", VoucherCode: "GL-UNMATCHED", VoucherDate: cashFlowTestDate,
				Lines: []CashFlowVoucherLine{tt.line},
			}
			allocations := AllocateCashFlow(voucher, map[string]bool{"1002": true})
			if len(allocations) != 1 {
				t.Fatalf("allocations = %d, want 1", len(allocations))
			}
			allocation := allocations[0]
			assertCashFlowAllocationSource(t, allocation, voucher)
			if allocation.ItemCode != tt.wantItem {
				t.Fatalf("item = %s, want %s", allocation.ItemCode, tt.wantItem)
			}
			if allocation.Direction != tt.wantDirection {
				t.Fatalf("direction = %s, want %s", allocation.Direction, tt.wantDirection)
			}
			if !allocation.Amount.Equal(decimal.NewFromInt(15)) {
				t.Fatalf("amount = %s, want 15", allocation.Amount)
			}
			if !allocation.Unclassified {
				t.Fatal("allocation must be unclassified")
			}
			if allocation.Reason != "无对方科目" {
				t.Fatalf("reason = %q, want %q", allocation.Reason, "无对方科目")
			}
		})
	}
}

func TestAllocateCashFlowUnclassified(t *testing.T) {
	voucher := CashFlowVoucher{
		VoucherID: "v4", VoucherCode: "GL-4", VoucherDate: cashFlowTestDate,
		Lines: []CashFlowVoucherLine{
			{AccountCode: "1002", Debit: decimal.NewFromInt(5)},
			{AccountCode: "4103", Credit: decimal.NewFromInt(5)},
		},
	}
	allocations := AllocateCashFlow(voucher, map[string]bool{"1002": true})
	if len(allocations) != 1 {
		t.Fatalf("allocations = %d, want 1", len(allocations))
	}
	allocation := allocations[0]
	assertCashFlowAllocationSource(t, allocation, voucher)
	if allocation.ItemCode != "other_operating_inflows" {
		t.Fatalf("item = %s, want other_operating_inflows", allocation.ItemCode)
	}
	if allocation.Direction != CashFlowInflow {
		t.Fatalf("direction = %s, want %s", allocation.Direction, CashFlowInflow)
	}
	if !allocation.Amount.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("amount = %s, want 5", allocation.Amount)
	}
	if !allocation.Unclassified {
		t.Fatal("allocation must be unclassified")
	}
	if allocation.Reason != "未识别科目 4103" {
		t.Fatalf("reason = %q, want %q", allocation.Reason, "未识别科目 4103")
	}
}

func TestAllocateCashFlowRoundingResidualNonNegative(t *testing.T) {
	voucher := CashFlowVoucher{
		VoucherID: "v-round", VoucherCode: "GL-ROUND", VoucherDate: cashFlowTestDate,
		Lines: []CashFlowVoucherLine{
			{AccountCode: "1002", Debit: decimal.RequireFromString("0.0002")},
			{AccountCode: "1122", Credit: decimal.NewFromInt(1)},
			{AccountCode: "1122", Credit: decimal.NewFromInt(1)},
			{AccountCode: "1122", Credit: decimal.NewFromInt(1)},
			{AccountCode: "1122", Credit: decimal.NewFromInt(1)},
		},
	}
	allocations := AllocateCashFlow(voucher, map[string]bool{"1002": true})
	if len(allocations) != 4 {
		t.Fatalf("allocations = %d, want 4", len(allocations))
	}
	total := decimal.Zero
	for _, allocation := range allocations {
		if allocation.Amount.IsNegative() {
			t.Fatalf("negative allocation: %s", allocation.Amount)
		}
		total = total.Add(allocation.Amount)
	}
	if !total.Equal(decimal.RequireFromString("0.0002")) {
		t.Fatalf("allocated total = %s, want 0.0002", total)
	}
}

func TestAllocateCashFlowNormalizesCashSubaccounts(t *testing.T) {
	tests := []struct {
		name        string
		cashAccount string
		cashCodes   map[string]bool
	}{
		{
			name:        "cash line subaccount",
			cashAccount: "1002.01",
			cashCodes:   map[string]bool{"1002": true},
		},
		{
			name:        "cash mapping subaccount",
			cashAccount: "1002",
			cashCodes:   map[string]bool{"1002.01": true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			voucher := CashFlowVoucher{
				VoucherID: "v-sub", VoucherCode: "GL-SUB", VoucherDate: cashFlowTestDate,
				Lines: []CashFlowVoucherLine{
					{AccountCode: tt.cashAccount, Debit: decimal.NewFromInt(50)},
					{AccountCode: "112201", Credit: decimal.NewFromInt(50)},
				},
			}
			allocations := AllocateCashFlow(voucher, tt.cashCodes)
			if len(allocations) != 1 {
				t.Fatalf("allocations = %d, want 1", len(allocations))
			}
			allocation := allocations[0]
			assertCashFlowAllocationSource(t, allocation, voucher)
			if allocation.ItemCode != "sales_receipts" {
				t.Fatalf("item = %s, want sales_receipts", allocation.ItemCode)
			}
			if !allocation.Amount.Equal(decimal.NewFromInt(50)) {
				t.Fatalf("amount = %s, want 50", allocation.Amount)
			}
			if allocation.Unclassified {
				t.Fatal("allocation must be classified")
			}
		})
	}
}

func assertCashFlowAllocationSource(t *testing.T, allocation CashFlowAllocation, voucher CashFlowVoucher) {
	t.Helper()
	if allocation.VoucherID != voucher.VoucherID {
		t.Fatalf("voucher id = %q, want %q", allocation.VoucherID, voucher.VoucherID)
	}
	if allocation.VoucherCode != voucher.VoucherCode {
		t.Fatalf("voucher code = %q, want %q", allocation.VoucherCode, voucher.VoucherCode)
	}
	if !allocation.VoucherDate.Equal(voucher.VoucherDate) {
		t.Fatalf("voucher date = %s, want %s", allocation.VoucherDate, voucher.VoucherDate)
	}
}
