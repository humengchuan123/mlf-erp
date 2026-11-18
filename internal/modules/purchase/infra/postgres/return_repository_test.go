package postgres

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestApplyPurchaseReturnCredit(t *testing.T) {
	tests := []struct {
		name              string
		returnTotal       string
		returnBase        string
		invoiceRate       string
		invoiceTotal      string
		invoicePaid       string
		invoiceCredited   string
		invoiceBaseTotal  string
		invoiceBasePaid   string
		invoiceBaseCredit string
		wantApplied       string
		wantAppliedBase   string
		wantRemaining     string
		wantRemainingBase string
		wantStatus        string
	}{
		{
			name:              "full application",
			returnTotal:       "100.0000",
			returnBase:        "100.0000",
			invoiceRate:       "1",
			invoiceTotal:      "100.0000",
			invoicePaid:       "0",
			invoiceCredited:   "0",
			invoiceBaseTotal:  "100.0000",
			invoiceBasePaid:   "0",
			invoiceBaseCredit: "0",
			wantApplied:       "100.0000",
			wantAppliedBase:   "100.0000",
			wantRemaining:     "0.0000",
			wantRemainingBase: "0.0000",
			wantStatus:        "applied",
		},
		{
			name:              "partial application against invoice outstanding",
			returnTotal:       "100.0000",
			returnBase:        "100.0000",
			invoiceRate:       "1",
			invoiceTotal:      "200.0000",
			invoicePaid:       "150.0000",
			invoiceCredited:   "0",
			invoiceBaseTotal:  "200.0000",
			invoiceBasePaid:   "150.0000",
			invoiceBaseCredit: "0",
			wantApplied:       "50.0000",
			wantAppliedBase:   "50.0000",
			wantRemaining:     "50.0000",
			wantRemainingBase: "50.0000",
			wantStatus:        "open",
		},
		{
			name:              "already paid invoice leaves open credit",
			returnTotal:       "100.0000",
			returnBase:        "100.0000",
			invoiceRate:       "1",
			invoiceTotal:      "100.0000",
			invoicePaid:       "100.0000",
			invoiceCredited:   "0",
			invoiceBaseTotal:  "100.0000",
			invoiceBasePaid:   "100.0000",
			invoiceBaseCredit: "0",
			wantApplied:       "0.0000",
			wantAppliedBase:   "0.0000",
			wantRemaining:     "100.0000",
			wantRemainingBase: "100.0000",
			wantStatus:        "open",
		},
		{
			// baseOutstanding = returnBase - 0.0001. The stored amounts must
			// still satisfy applied_base + remaining_base = base_amount even
			// though the note is marked applied.
			name:              "base rounding residual keeps invariant",
			returnTotal:       "100.0000",
			returnBase:        "100.0000",
			invoiceRate:       "1",
			invoiceTotal:      "100.0000",
			invoicePaid:       "0",
			invoiceCredited:   "0",
			invoiceBaseTotal:  "100.0000",
			invoiceBasePaid:   "0",
			invoiceBaseCredit: "0.0001",
			wantApplied:       "100.0000",
			wantAppliedBase:   "99.9999",
			wantRemaining:     "0.0000",
			wantRemainingBase: "0.0001",
			wantStatus:        "applied",
		},
		{
			name:              "foreign currency partial application",
			returnTotal:       "100.0000",
			returnBase:        "720.0000",
			invoiceRate:       "7.2",
			invoiceTotal:      "200.0000",
			invoicePaid:       "80.0000",
			invoiceCredited:   "0",
			invoiceBaseTotal:  "1440.0000",
			invoiceBasePaid:   "576.0000",
			invoiceBaseCredit: "0",
			wantApplied:       "100.0000",
			wantAppliedBase:   "720.0000",
			wantRemaining:     "0.0000",
			wantRemainingBase: "0.0000",
			wantStatus:        "applied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			returnTotal := decimal.RequireFromString(tt.returnTotal)
			returnBase := decimal.RequireFromString(tt.returnBase)
			got := applyPurchaseReturnCredit(
				returnTotal,
				returnBase,
				decimal.RequireFromString(tt.invoiceRate),
				decimal.RequireFromString(tt.invoiceTotal),
				decimal.RequireFromString(tt.invoicePaid),
				decimal.RequireFromString(tt.invoiceCredited),
				decimal.RequireFromString(tt.invoiceBaseTotal),
				decimal.RequireFromString(tt.invoiceBasePaid),
				decimal.RequireFromString(tt.invoiceBaseCredit),
			)

			if want := decimal.RequireFromString(tt.wantApplied); !got.Applied.Equal(want) {
				t.Fatalf("applied = %s, want %s", got.Applied, want)
			}
			if want := decimal.RequireFromString(tt.wantAppliedBase); !got.AppliedBase.Equal(want) {
				t.Fatalf("applied base = %s, want %s", got.AppliedBase, want)
			}
			if want := decimal.RequireFromString(tt.wantRemaining); !got.Remaining.Equal(want) {
				t.Fatalf("remaining = %s, want %s", got.Remaining, want)
			}
			if want := decimal.RequireFromString(tt.wantRemainingBase); !got.RemainingBase.Equal(want) {
				t.Fatalf("remaining base = %s, want %s", got.RemainingBase, want)
			}
			if got.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", got.Status, tt.wantStatus)
			}
			if !got.Applied.Add(got.Remaining).Equal(returnTotal) {
				t.Fatalf("applied + remaining = %s, want %s", got.Applied.Add(got.Remaining), returnTotal)
			}
			if !got.AppliedBase.Add(got.RemainingBase).Equal(returnBase) {
				t.Fatalf("applied base + remaining base = %s, want %s",
					got.AppliedBase.Add(got.RemainingBase), returnBase)
			}
		})
	}
}
