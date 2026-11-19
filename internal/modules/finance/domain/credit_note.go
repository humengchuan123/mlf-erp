package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

const (
	CreditNoteStatusOpen    = "open"
	CreditNoteStatusApplied = "applied"

	CreditNoteTypeSalesReturn    = "sales_return"
	CreditNoteTypePurchaseReturn = "purchase_return"
)

type CreditNote struct {
	ID                  string
	TenantID            string
	OrgID               string
	Code                string
	CreditType          string
	SourceType          string
	SourceID            string
	SourceCode          string
	InvoiceID           string
	InvoiceCode         string
	PartnerID           string
	PartnerName         string
	NoteDate            time.Time
	Currency            string
	ExchangeRate        decimal.Decimal
	Amount              decimal.Decimal
	BaseAmount          decimal.Decimal
	AppliedAmount       decimal.Decimal
	AppliedBaseAmount   decimal.Decimal
	RefundedAmount      decimal.Decimal
	RefundedBaseAmount  decimal.Decimal
	RemainingAmount     decimal.Decimal
	RemainingBaseAmount decimal.Decimal
	Status              string
	Reason              string
	Remark              string
	CreatedBy           string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Allocations         []*CreditNoteAllocation
	Refunds             []*CreditNoteRefund
}

func (c *CreditNote) Remaining() decimal.Decimal {
	return c.Amount.Sub(c.AppliedAmount).Sub(c.RefundedAmount).Round(4)
}

type CreditNoteAllocation struct {
	ID             string
	TenantID       string
	OrgID          string
	CreditNoteID   string
	InvoiceID      string
	InvoiceCode    string
	InvoiceDate    time.Time
	Amount         decimal.Decimal
	BaseAmount     decimal.Decimal
	AllocationDate time.Time
	CreatedBy      string
	CreatedAt      time.Time
}

type CreditNoteRefund struct {
	ID             string
	TenantID       string
	OrgID          string
	Code           string
	CreditNoteID   string
	RefundDate     time.Time
	Status         string
	Currency       string
	ExchangeRate   decimal.Decimal
	Amount         decimal.Decimal
	BaseAmount     decimal.Decimal
	CashBaseAmount decimal.Decimal
	FXAdjustment   decimal.Decimal
	Method         string
	Reference      string
	Remark         string
	VoucherCode    string
	CreatedBy      string
	CreatedAt      time.Time
}

type RefundInput struct {
	RefundDate   time.Time
	Amount       decimal.Decimal
	ExchangeRate decimal.Decimal
	Method       string
	Reference    string
	Remark       string
}
