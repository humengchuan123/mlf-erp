package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type Receipt struct {
	ID               string
	TenantID         string
	OrgID            string
	Code             string
	OrderID          string
	OrderCode        string
	SupplierName     string
	WarehouseID      string
	WarehouseName    string
	ReceiptDate      time.Time
	Status           string
	PriceIncludesTax bool
	NetAmount        decimal.Decimal
	TaxAmount        decimal.Decimal
	GrossAmount      decimal.Decimal
	Remark           string
	CreatedBy        string
	CreatedAt        time.Time
	PayableID        string
	PayableCode      string
	PayableDueDate   time.Time
	VoucherCode      string
	Lines            []*ReceiptLine
}

type ReceiptLine struct {
	ID           string
	TenantID     string
	ReceiptID    string
	OrderLineID  string
	MaterialID   string
	MaterialName string
	Quantity     decimal.Decimal
	UnitPrice    decimal.Decimal
	TaxRateID    string
	TaxRate      decimal.Decimal
	Amount       decimal.Decimal
	TaxAmount    decimal.Decimal
	GrossAmount  decimal.Decimal
}

func (r *Receipt) TotalAmount() decimal.Decimal {
	if !r.GrossAmount.IsZero() {
		return r.GrossAmount.Round(4)
	}
	total := decimal.Zero
	for _, line := range r.Lines {
		total = total.Add(line.GrossAmount)
	}
	return total.Round(4)
}
