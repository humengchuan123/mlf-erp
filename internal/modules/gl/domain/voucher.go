package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

const (
	VoucherStatusDraft    = "draft"
	VoucherStatusPosted   = "posted"
	VoucherStatusReversed = "reversed"

	VoucherTypeManual              = "manual"
	VoucherTypeSalesShipment       = "sales_shipment"
	VoucherTypePurchaseReceipt     = "purchase_receipt"
	VoucherTypeReceiptPayment      = "receipt_payment"
	VoucherTypeSupplierPayment     = "supplier_payment"
	VoucherTypeReversal            = "reversal"
	VoucherTypePeriodClose         = "period_close"
	VoucherTypePeriodCloseReversal = "period_close_reversal"
	VoucherTypeYearClose           = "year_close"

	SourceTypeManual              = "manual"
	SourceTypeSalesShipment       = "sales_shipment"
	SourceTypePurchaseReceipt     = "purchase_receipt"
	SourceTypeReceiptPayment      = "receipt_payment"
	SourceTypeSupplierPayment     = "supplier_payment"
	SourceTypeReversal            = "reversal"
	SourceTypePeriodClose         = "period_close"
	SourceTypePeriodCloseReversal = "period_close_reversal"
	SourceTypeYearClose           = "year_close"
)

type Voucher struct {
	ID                  string
	TenantID            string
	OrgID               string
	Code                string
	VoucherDate         time.Time
	PeriodID            string
	VoucherType         string
	SourceType          string
	SourceID            string
	SourceCode          string
	Status              string
	Memo                string
	CreatedBy           string
	PostedBy            string
	PostedAt            *time.Time
	ReversalOfVoucherID string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	TotalAmount         decimal.Decimal
	Lines               []*VoucherLine
	InputLines          []VoucherLineInput
}

type VoucherLine struct {
	ID           string
	TenantID     string
	VoucherID    string
	LineNo       int
	AccountID    string
	AccountCode  string
	AccountName  string
	Summary      string
	Debit        decimal.Decimal
	Credit       decimal.Decimal
	PartnerID    string
	DepartmentID string
	ProjectCode  string
}

type VoucherLineInput struct {
	AccountID    string
	Summary      string
	Debit        decimal.Decimal
	Credit       decimal.Decimal
	PartnerID    string
	DepartmentID string
	ProjectCode  string
}

type VoucherInput struct {
	VoucherDate time.Time
	Memo        string
	Lines       []VoucherLineInput
}

func (v *Voucher) TotalDebit() decimal.Decimal {
	total := decimal.Zero
	for _, line := range v.Lines {
		total = total.Add(line.Debit)
	}
	return total
}

func (v *Voucher) TotalCredit() decimal.Decimal {
	total := decimal.Zero
	for _, line := range v.Lines {
		total = total.Add(line.Credit)
	}
	return total
}
