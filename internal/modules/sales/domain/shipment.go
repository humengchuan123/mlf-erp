package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type Shipment struct {
	ID               string
	TenantID         string
	OrgID            string
	Code             string
	OrderID          string
	OrderCode        string
	CustomerID       string
	CustomerName     string
	WarehouseID      string
	WarehouseName    string
	ShipmentDate     time.Time
	Status           string
	PriceIncludesTax bool
	ExchangeRate     decimal.Decimal
	NetAmount        decimal.Decimal
	TaxAmount        decimal.Decimal
	GrossAmount      decimal.Decimal
	BaseNetAmount    decimal.Decimal
	BaseTaxAmount    decimal.Decimal
	BaseTotalAmount  decimal.Decimal
	Remark           string
	CreatedBy        string
	CreatedAt        time.Time
	InvoiceID        string
	InvoiceCode      string
	InvoiceDueDate   time.Time
	VoucherCode      string
	Lines            []*ShipmentLine
}

type ShipmentLine struct {
	ID              string
	TenantID        string
	ShipmentID      string
	OrderLineID     string
	MaterialID      string
	MaterialName    string
	Quantity        decimal.Decimal
	UnitPrice       decimal.Decimal
	TaxRateID       string
	TaxRate         decimal.Decimal
	Amount          decimal.Decimal
	TaxAmount       decimal.Decimal
	GrossAmount     decimal.Decimal
	BaseAmount      decimal.Decimal
	BaseTaxAmount   decimal.Decimal
	BaseGrossAmount decimal.Decimal
	UnitCost        decimal.Decimal
	CostAmount      decimal.Decimal
}

func (s *Shipment) TotalAmount() decimal.Decimal {
	if !s.GrossAmount.IsZero() {
		return s.GrossAmount.Round(4)
	}
	total := decimal.Zero
	for _, line := range s.Lines {
		total = total.Add(line.GrossAmount)
	}
	return total.Round(4)
}

func (s *Shipment) TotalCost() decimal.Decimal {
	total := decimal.Zero
	for _, line := range s.Lines {
		total = total.Add(line.CostAmount)
	}
	return total.Round(4)
}
