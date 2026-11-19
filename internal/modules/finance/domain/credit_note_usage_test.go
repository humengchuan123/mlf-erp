package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestAllocateBase(t *testing.T) {
	tests := []struct {
		remainingAmount, remainingBase, amount, want string
	}{
		{"100.0000", "720.0000", "50.0000", "360.0000"},
		{"3.0000", "100.0000", "1.0000", "33.3333"},
		{"3.0000", "100.0000", "2.0000", "66.6667"},
		{"3.0000", "100.0000", "3.0000", "100.0000"},
	}
	for _, tt := range tests {
		got := AllocateBase(decimal.RequireFromString(tt.remainingAmount),
			decimal.RequireFromString(tt.remainingBase),
			decimal.RequireFromString(tt.amount))
		if want := decimal.RequireFromString(tt.want); !got.Equal(want) {
			t.Fatalf("base = %s, want %s", got, want)
		}
	}
}

func TestCashBaseAndFXAdjustment(t *testing.T) {
	cash := CashBase(decimal.RequireFromString("100"), decimal.RequireFromString("7.3"))
	if !cash.Equal(decimal.RequireFromString("730.0000")) {
		t.Fatalf("cash base = %s, want 730.0000", cash)
	}
	if got := FXAdjustment(cash, decimal.RequireFromString("720.0000")); !got.Equal(decimal.RequireFromString("10.0000")) {
		t.Fatalf("fx adjustment = %s, want 10.0000", got)
	}
}

func TestCreditNoteStatus(t *testing.T) {
	if got := CreditNoteStatus(decimal.Zero, decimal.Zero); got != CreditNoteStatusApplied {
		t.Fatalf("status = %q, want applied", got)
	}
	if got := CreditNoteStatus(decimal.Zero, decimal.RequireFromString("0.0001")); got != CreditNoteStatusApplied {
		t.Fatalf("residual status = %q, want applied", got)
	}
	if got := CreditNoteStatus(decimal.RequireFromString("1"), decimal.Zero); got != CreditNoteStatusOpen {
		t.Fatalf("status = %q, want open", got)
	}
}
