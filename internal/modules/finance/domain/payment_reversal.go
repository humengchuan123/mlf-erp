package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type PaymentAllocation struct {
	ID                string
	PaymentID         string
	InvoiceID         string
	InvoiceCode       string
	InvoiceDate       time.Time
	Amount            decimal.Decimal
	BaseAmount        decimal.Decimal
	InvoiceBaseAmount decimal.Decimal
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
	FXReversalVoucher string
	CreatedBy         string
	CreatedAt         time.Time
}

type UnallocateInput struct {
	InvoiceIDs   []string
	ReversalDate time.Time
	Reason       string
}

func PaymentStatusAfterReversal(amount, allocated decimal.Decimal) string {
	if !allocated.IsPositive() {
		return PaymentStatusUnallocated
	}
	if allocated.LessThan(amount) {
		return PaymentStatusPartiallyAllocated
	}
	return PaymentStatusAllocated
}

func InvoiceStatusAfterSettlement(total, paid, credited decimal.Decimal) string {
	settled := paid.Add(credited)
	if settled.IsZero() {
		return InvoiceStatusOpen
	}
	if settled.GreaterThanOrEqual(total) {
		return InvoiceStatusPaid
	}
	return InvoiceStatusPartiallyPaid
}
