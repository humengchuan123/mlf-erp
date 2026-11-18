package domain

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestClassifyCashFlow(t *testing.T) {
	tests := []struct {
		code             string
		inflow           bool
		wantCode         string
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
