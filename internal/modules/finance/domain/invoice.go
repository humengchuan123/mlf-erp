package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

const (
	InvoiceTypeReceivable = "receivable"
	InvoiceTypePayable    = "payable"

	SourceTypeSalesShipment   = "sales_shipment"
	SourceTypePurchaseReceipt = "purchase_receipt"

	InvoiceStatusOpen          = "open"
	InvoiceStatusPartiallyPaid = "partially_paid"
	InvoiceStatusPaid          = "paid"
)

type Invoice struct {
	ID          string
	TenantID    string
	OrgID       string
	Code        string
	InvoiceType string
	SourceType  string
	SourceID    string
	SourceCode  string
	PartnerID   string
	PartnerName string
	InvoiceDate time.Time
	DueDate     time.Time
	Currency    string
	Status      string
	TotalAmount decimal.Decimal
	PaidAmount  decimal.Decimal
	Remark      string
	CreatedBy   string
	UpdatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (i *Invoice) Outstanding() decimal.Decimal {
	return i.TotalAmount.Sub(i.PaidAmount)
}

type Allocation struct {
	ID          string
	TenantID    string
	PaymentID   string
	InvoiceID   string
	Amount      decimal.Decimal
	CreatedBy   string
	CreatedAt   time.Time
	PaymentCode string
	PaymentType string
	PaymentDate time.Time
}
