package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// PaymentAllocation is one active allocation of a payment to an invoice.
// BaseAmount is the allocated amount converted at the payment's exchange rate;
// InvoiceBaseAmount is the allocated amount converted at the invoice's exchange
// rate.
type PaymentAllocation struct {
	ID                string
	TenantID          string
	PaymentID         string
	InvoiceID         string
	InvoiceCode       string
	InvoiceDate       time.Time
	Amount            decimal.Decimal
	BaseAmount        decimal.Decimal
	InvoiceBaseAmount decimal.Decimal
	CreatedBy         string
	CreatedAt         time.Time
}

type PaymentAllocationReversal struct {
	ID                string
	TenantID          string
	OrgID             string
	PaymentID         string
	PaymentCode       string
	PaymentDate       time.Time
	InvoiceID         string
	InvoiceCode       string
	InvoiceDate       time.Time
	AllocationID      string
	Amount            decimal.Decimal
	BaseAmount        decimal.Decimal
	InvoiceBaseAmount decimal.Decimal
	ReversalDate      time.Time
	Reason            string
	// FXReversalVoucherCodes lists one voucher code per reversed FX settlement
	// voucher. It is empty when there is no FX settlement or no voucher.
	FXReversalVoucherCodes []string
	CreatedBy              string
	CreatedAt              time.Time
}

type UnallocateInput struct {
	InvoiceIDs   []string
	ReversalDate time.Time
	Reason       string
}

// PaymentStatusAfterReversal derives the payment status from the remaining
// allocated amount after a reversal. amount and allocated are monetary
// decimals; negative allocated values are treated as zero (unallocated) as a
// defensive fallback.
func PaymentStatusAfterReversal(amount, allocated decimal.Decimal) string {
	if !allocated.IsPositive() {
		return PaymentStatusUnallocated
	}
	if allocated.LessThan(amount) {
		return PaymentStatusPartiallyAllocated
	}
	return PaymentStatusAllocated
}

// InvoiceStatusAfterSettlement derives the invoice status from total, paid and
// credited monetary decimals. It delegates to Invoice.StatusAfterSettlement so
// allocation and reversal share the same rounding, zero-total and
// over-settlement rules.
func InvoiceStatusAfterSettlement(total, paid, credited decimal.Decimal) string {
	invoice := Invoice{
		TotalAmount:    total,
		PaidAmount:     paid,
		CreditedAmount: credited,
	}
	return invoice.StatusAfterSettlement()
}
