package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestAllocateBase(t *testing.T) {
	tests := []struct {
		name                                         string
		remainingAmount, remainingBase, amount, want string
	}{
		{"proportional", "100.0000", "720.0000", "50.0000", "360.0000"},
		{"one third rounds", "3.0000", "100.0000", "1.0000", "33.3333"},
		{"two thirds rounds", "3.0000", "100.0000", "2.0000", "66.6667"},
		{"full allocation uses remaining base", "3.0000", "100.0000", "3.0000", "100.0000"},
		{"single rounded division avoids double rounding", "1000000000", "0.0001", "499999999.9999", "0.0000"},
		{"zero remaining amount", "0", "100.0000", "1.0000", "0"},
		{"zero amount", "10.0000", "100.0000", "0", "0"},
		{"negative amount", "10.0000", "100.0000", "-1", "0"},
		{"zero remaining base", "10.0000", "0", "1.0000", "0"},
		{"amount above remaining clamps to remaining base", "3.0000", "100.0000", "4.0000", "100.0000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AllocateBase(decimal.RequireFromString(tt.remainingAmount),
				decimal.RequireFromString(tt.remainingBase),
				decimal.RequireFromString(tt.amount))
			if want := decimal.RequireFromString(tt.want); !got.Equal(want) {
				t.Fatalf("base = %s, want %s", got, want)
			}
		})
	}
}

func TestAllocateBaseTelescopes(t *testing.T) {
	remainingAmount := decimal.RequireFromString("3")
	remainingBase := decimal.RequireFromString("100")
	wants := []string{"33.3333", "33.3334", "33.3333"}
	total := decimal.Zero
	for i, want := range wants {
		base := AllocateBase(remainingAmount, remainingBase, decimal.RequireFromString("1"))
		if !base.Equal(decimal.RequireFromString(want)) {
			t.Fatalf("step %d base = %s, want %s", i+1, base, want)
		}
		remainingAmount = remainingAmount.Sub(decimal.RequireFromString("1"))
		remainingBase = remainingBase.Sub(base)
		total = total.Add(base)
	}
	if !total.Equal(decimal.RequireFromString("100.0000")) {
		t.Fatalf("total base = %s, want 100.0000", total)
	}
	if !remainingAmount.IsZero() || !remainingBase.IsZero() {
		t.Fatalf("residual amount/base = %s/%s, want 0/0", remainingAmount, remainingBase)
	}
}

func TestCashBaseAndFXAdjustment(t *testing.T) {
	t.Run("cash base", func(t *testing.T) {
		cash := CashBase(decimal.RequireFromString("100"), decimal.RequireFromString("7.3"))
		if !cash.Equal(decimal.RequireFromString("730.0000")) {
			t.Fatalf("cash base = %s, want 730.0000", cash)
		}
	})

	t.Run("positive fx adjustment", func(t *testing.T) {
		if got := FXAdjustment(decimal.RequireFromString("730"), decimal.RequireFromString("720.0000")); !got.Equal(decimal.RequireFromString("10.0000")) {
			t.Fatalf("fx adjustment = %s, want 10.0000", got)
		}
	})

	t.Run("negative fx adjustment", func(t *testing.T) {
		if got := FXAdjustment(decimal.RequireFromString("720"), decimal.RequireFromString("730.0000")); !got.Equal(decimal.RequireFromString("-10.0000")) {
			t.Fatalf("fx adjustment = %s, want -10.0000", got)
		}
	})

	t.Run("cash base rounds to zero", func(t *testing.T) {
		if got := CashBase(decimal.RequireFromString("0.0001"), decimal.RequireFromString("0.4")); !got.IsZero() {
			t.Fatalf("cash base = %s, want 0", got)
		}
	})
}

func TestCreditNoteStatus(t *testing.T) {
	tests := []struct {
		name                                 string
		remainingAmount, remainingBase, want string
	}{
		{"fully used", "0", "0", CreditNoteStatusApplied},
		{"base residual within tolerance", "0", "0.0001", CreditNoteStatusApplied},
		{"negative base residual within tolerance", "0", "-0.0001", CreditNoteStatusApplied},
		{"base residual above tolerance stays applied", "0", "0.0002", CreditNoteStatusApplied},
		{"large positive base residual stays applied", "0", "10", CreditNoteStatusApplied},
		{"remaining amount", "1", "0", CreditNoteStatusOpen},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CreditNoteStatus(decimal.RequireFromString(tt.remainingAmount),
				decimal.RequireFromString(tt.remainingBase))
			if got != tt.want {
				t.Fatalf("status = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreditNoteRemainingIncludesRefunded(t *testing.T) {
	note := &CreditNote{
		Amount:         decimal.RequireFromString("100"),
		AppliedAmount:  decimal.RequireFromString("30"),
		RefundedAmount: decimal.RequireFromString("20"),
	}
	if got := note.Remaining(); !got.Equal(decimal.RequireFromString("50")) {
		t.Fatalf("remaining = %s, want 50", got)
	}
}
