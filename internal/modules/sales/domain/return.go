package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type SalesReturn struct {
	ID                  string
	TenantID            string
	OrgID               string
	Code                string
	ShipmentID          string
	ShipmentCode        string
	OrderID             string
	OrderCode           string
	CustomerID          string
	CustomerName        string
	WarehouseID         string
	WarehouseName       string
	ReturnDate          time.Time
	Status              string
	Currency            string
	ExchangeRate        decimal.Decimal
	PriceIncludesTax    bool
	NetAmount           decimal.Decimal
	TaxAmount           decimal.Decimal
	TotalAmount         decimal.Decimal
	BaseNetAmount       decimal.Decimal
	BaseTaxAmount       decimal.Decimal
	BaseTotalAmount     decimal.Decimal
	CostAmount          decimal.Decimal
	InvoiceID           string
	InvoiceCode         string
	CreditNoteID        string
	CreditNoteCode      string
	CreditNoteAmount    decimal.Decimal
	CreditNoteApplied   decimal.Decimal
	CreditNoteRemaining decimal.Decimal
	CreditNoteStatus    string
	VoucherCode         string
	Reason              string
	Remark              string
	CreatedBy           string
	CreatedAt           time.Time
	Lines               []*SalesReturnLine
}

type SalesReturnLine struct {
	ID              string
	TenantID        string
	ReturnID        string
	LineNo          int
	ShipmentLineID  string
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

type ReturnLineInput struct {
	ShipmentLineID string
	Quantity       decimal.Decimal
}

type ReturnCreateParams struct {
	ShipmentID     string
	ReturnDate     time.Time
	Reason         string
	Remark         string
	Lines          []ReturnLineInput
	ReturnCode     string
	CreditNoteCode string
	VoucherCode    string
}

func BuildReturnLines(shipment *Shipment, inputs []ReturnLineInput) ([]*SalesReturnLine, error) {
	if shipment == nil || len(inputs) == 0 {
		return nil, ErrInvalidInput
	}

	shipmentLines := make(map[string]*ShipmentLine, len(shipment.Lines))
	for _, line := range shipment.Lines {
		if line == nil {
			continue
		}
		shipmentLines[line.ID] = line
	}

	seen := make(map[string]struct{}, len(inputs))
	lines := make([]*SalesReturnLine, 0, len(inputs))
	for i, input := range inputs {
		if input.ShipmentLineID == "" {
			return nil, ErrInvalidInput
		}
		if _, exists := seen[input.ShipmentLineID]; exists {
			return nil, ErrInvalidInput
		}
		seen[input.ShipmentLineID] = struct{}{}

		shipmentLine, ok := shipmentLines[input.ShipmentLineID]
		if !ok {
			return nil, ErrShipmentLineNotFound
		}
		if !input.Quantity.IsPositive() || !shipmentLine.Quantity.IsPositive() {
			return nil, ErrInvalidInput
		}
		remaining := shipmentLine.Quantity.Sub(shipmentLine.ReturnedQty)
		if input.Quantity.GreaterThan(remaining) {
			return nil, ErrOverReturn
		}

		quantity := shipmentLine.Quantity
		priorQty := shipmentLine.ReturnedQty
		newQty := priorQty.Add(input.Quantity)

		priorNet := shipmentLine.Amount.Mul(priorQty).Div(quantity).Round(4)
		newNet := shipmentLine.Amount.Mul(newQty).Div(quantity).Round(4)
		amount := newNet.Sub(priorNet)

		priorGross := shipmentLine.GrossAmount.Mul(priorQty).Div(quantity).Round(4)
		newGross := shipmentLine.GrossAmount.Mul(newQty).Div(quantity).Round(4)
		gross := newGross.Sub(priorGross)
		tax := gross.Sub(amount)

		priorBaseNet := shipmentLine.BaseAmount.Mul(priorQty).Div(quantity).Round(4)
		newBaseNet := shipmentLine.BaseAmount.Mul(newQty).Div(quantity).Round(4)
		baseAmount := newBaseNet.Sub(priorBaseNet)

		priorBaseGross := shipmentLine.BaseGrossAmount.Mul(priorQty).Div(quantity).Round(4)
		newBaseGross := shipmentLine.BaseGrossAmount.Mul(newQty).Div(quantity).Round(4)
		baseGross := newBaseGross.Sub(priorBaseGross)
		baseTax := baseGross.Sub(baseAmount)

		priorCost := shipmentLine.CostAmount.Mul(priorQty).Div(quantity).Round(4)
		newCost := shipmentLine.CostAmount.Mul(newQty).Div(quantity).Round(4)
		cost := newCost.Sub(priorCost)

		lines = append(lines, &SalesReturnLine{
			TenantID:        shipmentLine.TenantID,
			LineNo:          i + 1,
			ShipmentLineID:  shipmentLine.ID,
			OrderLineID:     shipmentLine.OrderLineID,
			MaterialID:      shipmentLine.MaterialID,
			MaterialName:    shipmentLine.MaterialName,
			Quantity:        input.Quantity,
			UnitPrice:       shipmentLine.UnitPrice,
			TaxRateID:       shipmentLine.TaxRateID,
			TaxRate:         shipmentLine.TaxRate,
			Amount:          amount,
			TaxAmount:       tax,
			GrossAmount:     gross,
			BaseAmount:      baseAmount,
			BaseTaxAmount:   baseTax,
			BaseGrossAmount: baseGross,
			UnitCost:        shipmentLine.UnitCost,
			CostAmount:      cost,
		})
	}
	return lines, nil
}

func (r *SalesReturn) RecalculateTotals() {
	net := decimal.Zero
	tax := decimal.Zero
	gross := decimal.Zero
	baseNet := decimal.Zero
	baseTax := decimal.Zero
	baseGross := decimal.Zero
	cost := decimal.Zero
	for _, line := range r.Lines {
		if line == nil {
			continue
		}
		net = net.Add(line.Amount)
		tax = tax.Add(line.TaxAmount)
		gross = gross.Add(line.GrossAmount)
		baseNet = baseNet.Add(line.BaseAmount)
		baseTax = baseTax.Add(line.BaseTaxAmount)
		baseGross = baseGross.Add(line.BaseGrossAmount)
		cost = cost.Add(line.CostAmount)
	}
	r.NetAmount = net.Round(4)
	r.TaxAmount = tax.Round(4)
	r.TotalAmount = gross.Round(4)
	r.BaseNetAmount = baseNet.Round(4)
	r.BaseTaxAmount = baseTax.Round(4)
	r.BaseTotalAmount = baseGross.Round(4)
	r.CostAmount = cost.Round(4)
}
