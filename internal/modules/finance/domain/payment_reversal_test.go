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
		{"over-allocated", "100", "120", PaymentStatusAllocated},
		{"negative allocated", "100", "-1", PaymentStatusUnallocated},
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
		{"credit only", "100", "0", "100", InvoiceStatusPaid},
		{"over-settled", "100", "80", "30", InvoiceStatusPaid},
		{"zero total", "0", "0", "0", InvoiceStatusPaid},
		{"rounding to zero outstanding", "10.00004", "10.00003", "0", InvoiceStatusPaid},
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
