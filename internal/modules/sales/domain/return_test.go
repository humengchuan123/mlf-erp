package domain

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func returnDec(value string) decimal.Decimal {
	return decimal.RequireFromString(value)
}

func newReturnTestShipment() *Shipment {
	return &Shipment{
		ID: "shipment-1",
		Lines: []*ShipmentLine{
			{
				ID:              "shipment-line-1",
				TenantID:        "tenant-1",
				OrderLineID:     "order-line-1",
				MaterialID:      "material-1",
				MaterialName:    "Material A",
				Quantity:        returnDec("10"),
				UnitPrice:       returnDec("100"),
				TaxRateID:       "tax-rate-1",
				TaxRate:         returnDec("0.13"),
				Amount:          returnDec("1000"),
				TaxAmount:       returnDec("130"),
				GrossAmount:     returnDec("1130"),
				BaseAmount:      returnDec("7200"),
				BaseTaxAmount:   returnDec("936"),
				BaseGrossAmount: returnDec("8136"),
				UnitCost:        returnDec("60"),
				CostAmount:      returnDec("600"),
			},
			{
				ID:              "shipment-line-2",
				TenantID:        "tenant-1",
				OrderLineID:     "order-line-2",
				MaterialID:      "material-2",
				MaterialName:    "Material B",
				Quantity:        returnDec("10"),
				UnitPrice:       returnDec("50"),
				TaxRate:         decimal.Zero,
				Amount:          returnDec("500"),
				TaxAmount:       decimal.Zero,
				GrossAmount:     returnDec("500"),
				BaseAmount:      returnDec("3600"),
				BaseTaxAmount:   decimal.Zero,
				BaseGrossAmount: returnDec("3600"),
				UnitCost:        decimal.Zero,
				CostAmount:      decimal.Zero,
			},
		},
	}
}

func assertReturnDecimal(t *testing.T, name string, got, want decimal.Decimal) {
	t.Helper()
	if !got.Equal(want) {
		t.Fatalf("%s = %s, want %s", name, got, want)
	}
}

func TestBuildReturnLinesPartial(t *testing.T) {
	lines, err := BuildReturnLines(newReturnTestShipment(), []ReturnLineInput{
		{ShipmentLineID: "shipment-line-1", Quantity: returnDec("2")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(lines))
	}

	line := lines[0]
	if line.LineNo != 1 {
		t.Fatalf("line no = %d, want 1", line.LineNo)
	}
	if line.ShipmentLineID != "shipment-line-1" ||
		line.OrderLineID != "order-line-1" ||
		line.MaterialID != "material-1" ||
		line.MaterialName != "Material A" {
		t.Fatalf("line source fields not copied: %+v", line)
	}
	assertReturnDecimal(t, "quantity", line.Quantity, returnDec("2"))
	assertReturnDecimal(t, "unit price", line.UnitPrice, returnDec("100"))
	assertReturnDecimal(t, "tax rate", line.TaxRate, returnDec("0.13"))
	assertReturnDecimal(t, "amount", line.Amount, returnDec("200"))
	assertReturnDecimal(t, "tax amount", line.TaxAmount, returnDec("26"))
	assertReturnDecimal(t, "gross amount", line.GrossAmount, returnDec("226"))
	assertReturnDecimal(t, "base amount", line.BaseAmount, returnDec("1440"))
	assertReturnDecimal(t, "base tax amount", line.BaseTaxAmount, returnDec("187.2"))
	assertReturnDecimal(t, "base gross amount", line.BaseGrossAmount, returnDec("1627.2"))
	assertReturnDecimal(t, "unit cost", line.UnitCost, returnDec("60"))
	assertReturnDecimal(t, "cost amount", line.CostAmount, returnDec("120"))
}

func TestBuildReturnLinesFullReturn(t *testing.T) {
	lines, err := BuildReturnLines(newReturnTestShipment(), []ReturnLineInput{
		{ShipmentLineID: "shipment-line-1", Quantity: returnDec("10")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}

	line := lines[0]
	assertReturnDecimal(t, "quantity", line.Quantity, returnDec("10"))
	assertReturnDecimal(t, "amount", line.Amount, returnDec("1000"))
	assertReturnDecimal(t, "tax amount", line.TaxAmount, returnDec("130"))
	assertReturnDecimal(t, "gross amount", line.GrossAmount, returnDec("1130"))
	assertReturnDecimal(t, "base amount", line.BaseAmount, returnDec("7200"))
	assertReturnDecimal(t, "base tax amount", line.BaseTaxAmount, returnDec("936"))
	assertReturnDecimal(t, "base gross amount", line.BaseGrossAmount, returnDec("8136"))
	assertReturnDecimal(t, "unit cost", line.UnitCost, returnDec("60"))
	assertReturnDecimal(t, "cost amount", line.CostAmount, returnDec("600"))
}

func TestBuildReturnLinesZeroTaxAndCost(t *testing.T) {
	lines, err := BuildReturnLines(newReturnTestShipment(), []ReturnLineInput{
		{ShipmentLineID: "shipment-line-2", Quantity: returnDec("5")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}

	line := lines[0]
	assertReturnDecimal(t, "amount", line.Amount, returnDec("250"))
	assertReturnDecimal(t, "tax amount", line.TaxAmount, decimal.Zero)
	assertReturnDecimal(t, "gross amount", line.GrossAmount, returnDec("250"))
	assertReturnDecimal(t, "base amount", line.BaseAmount, returnDec("1800"))
	assertReturnDecimal(t, "base tax amount", line.BaseTaxAmount, decimal.Zero)
	assertReturnDecimal(t, "base gross amount", line.BaseGrossAmount, returnDec("1800"))
	assertReturnDecimal(t, "unit cost", line.UnitCost, decimal.Zero)
	assertReturnDecimal(t, "cost amount", line.CostAmount, decimal.Zero)
}

func TestBuildReturnLinesRemainingQuantity(t *testing.T) {
	shipment := newReturnTestShipment()
	shipment.Lines[0].ReturnedQty = returnDec("8")

	lines, err := BuildReturnLines(shipment, []ReturnLineInput{
		{ShipmentLineID: "shipment-line-1", Quantity: returnDec("2")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}
	assertReturnDecimal(t, "quantity", lines[0].Quantity, returnDec("2"))
	assertReturnDecimal(t, "amount", lines[0].Amount, returnDec("200"))
}

func TestBuildReturnLinesErrors(t *testing.T) {
	tests := []struct {
		name     string
		shipment func() *Shipment
		inputs   []ReturnLineInput
		wantErr  error
	}{
		{
			name:     "nil shipment",
			shipment: func() *Shipment { return nil },
			inputs:   []ReturnLineInput{{ShipmentLineID: "shipment-line-1", Quantity: returnDec("1")}},
			wantErr:  ErrInvalidInput,
		},
		{
			name:     "no inputs",
			shipment: newReturnTestShipment,
			inputs:   nil,
			wantErr:  ErrInvalidInput,
		},
		{
			name:     "empty shipment line id",
			shipment: newReturnTestShipment,
			inputs:   []ReturnLineInput{{Quantity: returnDec("1")}},
			wantErr:  ErrInvalidInput,
		},
		{
			name:     "unknown shipment line",
			shipment: newReturnTestShipment,
			inputs:   []ReturnLineInput{{ShipmentLineID: "missing", Quantity: returnDec("1")}},
			wantErr:  ErrInvalidInput,
		},
		{
			name:     "zero quantity",
			shipment: newReturnTestShipment,
			inputs:   []ReturnLineInput{{ShipmentLineID: "shipment-line-1", Quantity: decimal.Zero}},
			wantErr:  ErrInvalidInput,
		},
		{
			name:     "negative quantity",
			shipment: newReturnTestShipment,
			inputs:   []ReturnLineInput{{ShipmentLineID: "shipment-line-1", Quantity: returnDec("-1")}},
			wantErr:  ErrInvalidInput,
		},
		{
			name:     "duplicate shipment line",
			shipment: newReturnTestShipment,
			inputs: []ReturnLineInput{
				{ShipmentLineID: "shipment-line-1", Quantity: returnDec("1")},
				{ShipmentLineID: "shipment-line-1", Quantity: returnDec("1")},
			},
			wantErr: ErrInvalidInput,
		},
		{
			name:     "zero shipment quantity",
			shipment: func() *Shipment { s := newReturnTestShipment(); s.Lines[0].Quantity = decimal.Zero; return s },
			inputs:   []ReturnLineInput{{ShipmentLineID: "shipment-line-1", Quantity: returnDec("1")}},
			wantErr:  ErrInvalidInput,
		},
		{
			name:     "over return",
			shipment: newReturnTestShipment,
			inputs:   []ReturnLineInput{{ShipmentLineID: "shipment-line-1", Quantity: returnDec("11")}},
			wantErr:  ErrOverReturn,
		},
		{
			name: "over return after previous returns",
			shipment: func() *Shipment {
				s := newReturnTestShipment()
				s.Lines[0].ReturnedQty = returnDec("8")
				return s
			},
			inputs:  []ReturnLineInput{{ShipmentLineID: "shipment-line-1", Quantity: returnDec("3")}},
			wantErr: ErrOverReturn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildReturnLines(tt.shipment(), tt.inputs)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildReturnLinesLineNumbers(t *testing.T) {
	lines, err := BuildReturnLines(newReturnTestShipment(), []ReturnLineInput{
		{ShipmentLineID: "shipment-line-2", Quantity: returnDec("5")},
		{ShipmentLineID: "shipment-line-1", Quantity: returnDec("2")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}
	if lines[0].LineNo != 1 || lines[1].LineNo != 2 {
		t.Fatalf("line numbers = %d, %d, want 1, 2", lines[0].LineNo, lines[1].LineNo)
	}
	if lines[0].ShipmentLineID != "shipment-line-2" || lines[1].ShipmentLineID != "shipment-line-1" {
		t.Fatalf("line order not preserved: %s, %s", lines[0].ShipmentLineID, lines[1].ShipmentLineID)
	}
}

func TestSalesReturnRecalculateTotals(t *testing.T) {
	lines, err := BuildReturnLines(newReturnTestShipment(), []ReturnLineInput{
		{ShipmentLineID: "shipment-line-1", Quantity: returnDec("2")},
		{ShipmentLineID: "shipment-line-2", Quantity: returnDec("5")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}

	salesReturn := &SalesReturn{Lines: lines}
	salesReturn.RecalculateTotals()

	assertReturnDecimal(t, "net amount", salesReturn.NetAmount, returnDec("450"))
	assertReturnDecimal(t, "tax amount", salesReturn.TaxAmount, returnDec("26"))
	assertReturnDecimal(t, "total amount", salesReturn.TotalAmount, returnDec("476"))
	assertReturnDecimal(t, "base net amount", salesReturn.BaseNetAmount, returnDec("3240"))
	assertReturnDecimal(t, "base tax amount", salesReturn.BaseTaxAmount, returnDec("187.2"))
	assertReturnDecimal(t, "base total amount", salesReturn.BaseTotalAmount, returnDec("3427.2"))
	assertReturnDecimal(t, "cost amount", salesReturn.CostAmount, returnDec("120"))
}
