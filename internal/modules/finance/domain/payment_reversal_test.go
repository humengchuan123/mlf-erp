package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestPaymentStatusAfterReversal(t *testing.T) {
	tests := []struct {
		name, amount, allocated, want string
	}{
		{"unallocated", "100", "0", PaymentStatusUnallocated},
		{"partial", "100", "40", PaymentStatusPartiallyAllocated},
		{"allocated", "100", "100", PaymentStatusAllocated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PaymentStatusAfterReversal(
				decimal.RequireFromString(tt.amount),
				decimal.RequireFromString(tt.allocated),
			)
			if got != tt.want {
				t.Fatalf("status = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInvoiceStatusAfterSettlement(t *testing.T) {
	tests := []struct {
		name, total, paid, credited, want string
	}{
		{"paid", "100", "40", "60", InvoiceStatusPaid},
		{"partial", "100", "20", "60", InvoiceStatusPartiallyPaid},
		{"open", "100", "0", "0", InvoiceStatusOpen},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InvoiceStatusAfterSettlement(
				decimal.RequireFromString(tt.total),
				decimal.RequireFromString(tt.paid),
				decimal.RequireFromString(tt.credited),
			)
			if got != tt.want {
				t.Fatalf("status = %q, want %q", got, tt.want)
			}
		})
	}
}
