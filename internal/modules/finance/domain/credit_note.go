package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

const (
	CreditNoteStatusOpen    = "open"
	CreditNoteStatusApplied = "applied"

	CreditNoteTypeSalesReturn = "sales_return"
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
	PartnerID           string
	PartnerName         string
	NoteDate            time.Time
	Currency            string
	ExchangeRate        decimal.Decimal
	Amount              decimal.Decimal
	BaseAmount          decimal.Decimal
	AppliedAmount       decimal.Decimal
	AppliedBaseAmount   decimal.Decimal
	RemainingAmount     decimal.Decimal
	RemainingBaseAmount decimal.Decimal
	Status              string
	Reason              string
	Remark              string
	CreatedBy           string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (c *CreditNote) Remaining() decimal.Decimal {
	return c.Amount.Sub(c.AppliedAmount).Round(4)
}
