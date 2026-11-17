package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

const (
	PaymentTypeReceipt = "receipt"
	PaymentTypePayment = "payment"

	PaymentStatusUnallocated        = "unallocated"
	PaymentStatusPartiallyAllocated = "partially_allocated"
	PaymentStatusAllocated          = "allocated"
)

type Payment struct {
	ID                  string
	TenantID            string
	OrgID               string
	Code                string
	PaymentType         string
	PartnerID           string
	PartnerName         string
	PaymentDate         time.Time
	Currency            string
	ExchangeRate        decimal.Decimal
	Amount              decimal.Decimal
	AllocatedAmount     decimal.Decimal
	BaseAmount          decimal.Decimal
	BaseAllocatedAmount decimal.Decimal
	Status              string
	Method              string
	Reference           string
	Remark              string
	CreatedBy           string
	CreatedAt           time.Time
	VoucherCode         string
}

func (p *Payment) Unallocated() decimal.Decimal {
	return p.Amount.Sub(p.AllocatedAmount)
}

type AllocationInput struct {
	InvoiceID string
	Amount    decimal.Decimal
}
