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
	ExchangeRate     decimal.Decimal
	Status           Status
	PriceIncludesTax bool
	NetAmount        decimal.Decimal
	TaxAmount        decimal.Decimal
	TotalAmount      decimal.Decimal
	BaseNetAmount    decimal.Decimal
	BaseTaxAmount    decimal.Decimal
	BaseTotalAmount  decimal.Decimal
	Remark           string
	CreatedBy        string
	UpdatedBy        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Lines            []*OrderLine
}

type OrderLine struct {
	ID              string
	TenantID        string
	OrderID         string
	LineNo          int
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
	ShippedQty      decimal.Decimal
}

func (l *OrderLine) Remaining() decimal.Decimal {
	return l.Quantity.Sub(l.ShippedQty)
}

func (o *Order) Recalculate() error {
	rate := o.ExchangeRate
	if !rate.IsPositive() {
		rate = decimal.NewFromInt(1)
	}
	netTotal := decimal.Zero
	taxTotal := decimal.Zero
	grossTotal := decimal.Zero
	baseNetTotal := decimal.Zero
	baseTaxTotal := decimal.Zero
	baseGrossTotal := decimal.Zero
	for _, line := range o.Lines {
		result, err := taxcalc.Calculate(line.Quantity, line.UnitPrice, line.TaxRate, o.PriceIncludesTax)
		if err != nil {
			return ErrInvalidInput
		}
		line.Amount = result.Net
		line.TaxAmount = result.Tax
		line.GrossAmount = result.Gross
		line.BaseAmount = result.Net.Mul(rate).Round(4)
		line.BaseGrossAmount = result.Gross.Mul(rate).Round(4)
		line.BaseTaxAmount = line.BaseGrossAmount.Sub(line.BaseAmount).Round(4)
		netTotal = netTotal.Add(result.Net)
		taxTotal = taxTotal.Add(result.Tax)
		grossTotal = grossTotal.Add(result.Gross)
		baseNetTotal = baseNetTotal.Add(line.BaseAmount)
		baseTaxTotal = baseTaxTotal.Add(line.BaseTaxAmount)
		baseGrossTotal = baseGrossTotal.Add(line.BaseGrossAmount)
	}
	o.NetAmount = netTotal.Round(4)
	o.TaxAmount = taxTotal.Round(4)
	o.TotalAmount = grossTotal.Round(4)
	o.BaseNetAmount = baseNetTotal.Round(4)
	o.BaseTaxAmount = baseTaxTotal.Round(4)
	o.BaseTotalAmount = baseGrossTotal.Round(4)
	return nil
}
