package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestInvoiceOutstandingAndStatusAfterSettlement(t *testing.T) {
	tests := []struct {
		name            string
		total           string
		paid            string
		credited        string
		wantOutstanding string
		wantStatus      string
	}{
		{
			name:            "unsettled",
			total:           "100",
			paid:            "0",
			credited:        "0",
			wantOutstanding: "100",
			wantStatus:      InvoiceStatusOpen,
		},
		{
			name:            "partially paid",
			total:           "100",
			paid:            "30",
			credited:        "0",
			wantOutstanding: "70",
			wantStatus:      InvoiceStatusPartiallyPaid,
		},
		{
			name:            "partially credited",
			total:           "100",
			paid:            "0",
			credited:        "30",
			wantOutstanding: "70",
			wantStatus:      InvoiceStatusPartiallyPaid,
		},
		{
			name:            "paid and credited settle exactly",
			total:           "100",
			paid:            "40",
			credited:        "60",
			wantOutstanding: "0",
			wantStatus:      InvoiceStatusPaid,
		},
		{
			name:            "credited settles in full",
			total:           "100",
			paid:            "0",
			credited:        "100",
			wantOutstanding: "0",
			wantStatus:      InvoiceStatusPaid,
		},
		{
			name:            "over settled is clamped",
			total:           "100",
			paid:            "110",
			credited:        "0",
			wantOutstanding: "0",
			wantStatus:      InvoiceStatusPaid,
		},
		{
			name:            "rounding settles exactly",
			total:           "10.00004",
			paid:            "10.00003",
			credited:        "0",
			wantOutstanding: "0",
			wantStatus:      InvoiceStatusPaid,
		},
		{
			name:            "rounding leaves a balance",
			total:           "10.00008",
			paid:            "10.00003",
			credited:        "0",
			wantOutstanding: "0.0001",
			wantStatus:      InvoiceStatusPartiallyPaid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoice := &Invoice{
				TotalAmount:    decimal.RequireFromString(tt.total),
				PaidAmount:     decimal.RequireFromString(tt.paid),
				CreditedAmount: decimal.RequireFromString(tt.credited),
			}
			wantOutstanding := decimal.RequireFromString(tt.wantOutstanding)
			if got := invoice.Outstanding(); !got.Equal(wantOutstanding) {
				t.Fatalf("outstanding = %s, want %s", got, wantOutstanding)
			}
			if got := invoice.StatusAfterSettlement(); got != tt.wantStatus {
				t.Fatalf("status = %q, want %q", got, tt.wantStatus)
			}
		})
	}
}
