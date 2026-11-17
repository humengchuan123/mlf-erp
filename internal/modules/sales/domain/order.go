package domain

import (
	"time"

	"github.com/mlf-erp/mlf-erp/internal/platform/taxcalc"
	"github.com/shopspring/decimal"
)

type Order struct {
	ID               string
	TenantID         string
	OrgID            string
	Code             string
	CustomerID       string
	CustomerName     string
	WarehouseID      string
	WarehouseName    string
	OrderDate        time.Time
	Currency         string
	Status           Status
	PriceIncludesTax bool
	NetAmount        decimal.Decimal
	TaxAmount        decimal.Decimal
	TotalAmount      decimal.Decimal
	Remark           string
	CreatedBy        string
	UpdatedBy        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Lines            []*OrderLine
}

type OrderLine struct {
	ID           string
	TenantID     string
	OrderID      string
	LineNo       int
	MaterialID   string
	MaterialName string
	Quantity     decimal.Decimal
	UnitPrice    decimal.Decimal
	TaxRateID    string
	TaxRate      decimal.Decimal
	Amount       decimal.Decimal
	TaxAmount    decimal.Decimal
	GrossAmount  decimal.Decimal
	ShippedQty   decimal.Decimal
}

func (l *OrderLine) Remaining() decimal.Decimal {
	return l.Quantity.Sub(l.ShippedQty)
}

func (o *Order) Recalculate() error {
	netTotal := decimal.Zero
	taxTotal := decimal.Zero
	grossTotal := decimal.Zero
	for _, line := range o.Lines {
		result, err := taxcalc.Calculate(line.Quantity, line.UnitPrice, line.TaxRate, o.PriceIncludesTax)
		if err != nil {
			return ErrInvalidInput
		}
		line.Amount = result.Net
		line.TaxAmount = result.Tax
		line.GrossAmount = result.Gross
		netTotal = netTotal.Add(result.Net)
		taxTotal = taxTotal.Add(result.Tax)
		grossTotal = grossTotal.Add(result.Gross)
	}
	o.NetAmount = netTotal.Round(4)
	o.TaxAmount = taxTotal.Round(4)
	o.TotalAmount = grossTotal.Round(4)
	return nil
}
