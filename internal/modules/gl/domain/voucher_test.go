package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestVoucherTotals(t *testing.T) {
	voucher := &Voucher{
		Lines: []*VoucherLine{
			{Debit: decimal.RequireFromString("100.00"), Credit: decimal.Zero},
			{Debit: decimal.Zero, Credit: decimal.RequireFromString("100.00")},
		},
	}
	if got := voucher.TotalDebit(); !got.Equal(decimal.RequireFromString("100")) {
		t.Fatalf("total debit = %s, want 100", got)
	}
	if got := voucher.TotalCredit(); !got.Equal(decimal.RequireFromString("100")) {
		t.Fatalf("total credit = %s, want 100", got)
	}
}

func TestVoucherTotalsCompoundEntry(t *testing.T) {
	voucher := &Voucher{
		Lines: []*VoucherLine{
			{Debit: decimal.RequireFromString("300.00")},
			{Debit: decimal.RequireFromString("100.00")},
			{Credit: decimal.RequireFromString("300.00")},
			{Credit: decimal.RequireFromString("100.00")},
		},
	}
	if !voucher.TotalDebit().Equal(voucher.TotalCredit()) {
		t.Fatalf("debit %s != credit %s", voucher.TotalDebit(), voucher.TotalCredit())
	}
}
