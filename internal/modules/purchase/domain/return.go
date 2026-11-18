package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type PurchaseReturn struct {
	ID                  string
	TenantID            string
	OrgID               string
	Code                string
	ReceiptID           string
	ReceiptCode         string
	OrderID             string
	OrderCode           string
	SupplierID          string
	SupplierName        string
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
	Lines               []*PurchaseReturnLine
}

type PurchaseReturnLine struct {
	ID              string
	TenantID        string
	ReturnID        string
	LineNo          int
	ReceiptLineID   string
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
	ReceiptLineID string
	Quantity      decimal.Decimal
}

type ReturnCreateParams struct {
	ReceiptID      string
	ReturnDate     time.Time
	Reason         string
	Remark         string
	Lines          []ReturnLineInput
	ReturnCode     string
	CreditNoteCode string
	VoucherCode    string
}

func BuildReturnLines(receipt *Receipt, inputs []ReturnLineInput) ([]*PurchaseReturnLine, error) {
	if receipt == nil || len(inputs) == 0 {
		return nil, ErrInvalidInput
	}

	receiptLines := make(map[string]*ReceiptLine, len(receipt.Lines))
	for _, line := range receipt.Lines {
		if line == nil {
			continue
		}
		receiptLines[line.ID] = line
	}

	seen := make(map[string]struct{}, len(inputs))
	lines := make([]*PurchaseReturnLine, 0, len(inputs))
	for i, input := range inputs {
		if input.ReceiptLineID == "" {
			return nil, ErrInvalidInput
		}
		if _, exists := seen[input.ReceiptLineID]; exists {
			return nil, ErrInvalidInput
		}
		seen[input.ReceiptLineID] = struct{}{}

		receiptLine, ok := receiptLines[input.ReceiptLineID]
		if !ok {
			return nil, ErrReceiptLineNotFound
		}
		if !input.Quantity.IsPositive() || !receiptLine.Quantity.IsPositive() {
			return nil, ErrInvalidInput
		}
		remaining := receiptLine.Quantity.Sub(receiptLine.ReturnedQty)
		if input.Quantity.GreaterThan(remaining) {
			return nil, ErrOverReturn
		}

		quantity := receiptLine.Quantity
		priorQty := receiptLine.ReturnedQty
		newQty := priorQty.Add(input.Quantity)

		priorNet := receiptLine.Amount.Mul(priorQty).Div(quantity).Round(4)
		newNet := receiptLine.Amount.Mul(newQty).Div(quantity).Round(4)
		amount := newNet.Sub(priorNet)

		priorGross := receiptLine.GrossAmount.Mul(priorQty).Div(quantity).Round(4)
		newGross := receiptLine.GrossAmount.Mul(newQty).Div(quantity).Round(4)
		gross := newGross.Sub(priorGross)
		tax := gross.Sub(amount)

		priorBaseNet := receiptLine.BaseAmount.Mul(priorQty).Div(quantity).Round(4)
		newBaseNet := receiptLine.BaseAmount.Mul(newQty).Div(quantity).Round(4)
		baseAmount := newBaseNet.Sub(priorBaseNet)

		priorBaseGross := receiptLine.BaseGrossAmount.Mul(priorQty).Div(quantity).Round(4)
		newBaseGross := receiptLine.BaseGrossAmount.Mul(newQty).Div(quantity).Round(4)
		baseGross := newBaseGross.Sub(priorBaseGross)
		baseTax := baseGross.Sub(baseAmount)

		// Purchase receipts store inventory value as the base net amount, so the
		// original unit cost is derived from it and returns leave inventory at
		// that same cost.
		unitCost := receiptLine.BaseAmount.Div(quantity).Round(6)
		priorCost := receiptLine.BaseAmount.Mul(priorQty).Div(quantity).Round(4)
		newCost := receiptLine.BaseAmount.Mul(newQty).Div(quantity).Round(4)
		cost := newCost.Sub(priorCost)

		lines = append(lines, &PurchaseReturnLine{
			TenantID:        receiptLine.TenantID,
			LineNo:          i + 1,
			ReceiptLineID:   receiptLine.ID,
			OrderLineID:     receiptLine.OrderLineID,
			MaterialID:      receiptLine.MaterialID,
			MaterialName:    receiptLine.MaterialName,
			Quantity:        input.Quantity,
			UnitPrice:       receiptLine.UnitPrice,
			TaxRateID:       receiptLine.TaxRateID,
			TaxRate:         receiptLine.TaxRate,
			Amount:          amount,
			TaxAmount:       tax,
			GrossAmount:     gross,
			BaseAmount:      baseAmount,
			BaseTaxAmount:   baseTax,
			BaseGrossAmount: baseGross,
			UnitCost:        unitCost,
			CostAmount:      cost,
		})
	}
	return lines, nil
}

func (r *PurchaseReturn) RecalculateTotals() {
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
