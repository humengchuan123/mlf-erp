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

func TestBuildReturnLinesZeroPricePositiveCost(t *testing.T) {
	shipment := newReturnTestShipment()
	shipmentLine := shipment.Lines[0]
	shipmentLine.Amount = decimal.Zero
	shipmentLine.TaxAmount = decimal.Zero
	shipmentLine.GrossAmount = decimal.Zero
	shipmentLine.BaseAmount = decimal.Zero
	shipmentLine.BaseTaxAmount = decimal.Zero
	shipmentLine.BaseGrossAmount = decimal.Zero

	lines, err := BuildReturnLines(shipment, []ReturnLineInput{
		{ShipmentLineID: shipmentLine.ID, Quantity: returnDec("4")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}

	line := lines[0]
	assertReturnDecimal(t, "amount", line.Amount, decimal.Zero)
	assertReturnDecimal(t, "tax amount", line.TaxAmount, decimal.Zero)
	assertReturnDecimal(t, "gross amount", line.GrossAmount, decimal.Zero)
	assertReturnDecimal(t, "base amount", line.BaseAmount, decimal.Zero)
	assertReturnDecimal(t, "base tax amount", line.BaseTaxAmount, decimal.Zero)
	assertReturnDecimal(t, "base gross amount", line.BaseGrossAmount, decimal.Zero)
	assertReturnDecimal(t, "unit cost", line.UnitCost, returnDec("60"))
	assertReturnDecimal(t, "cost amount", line.CostAmount, returnDec("240"))

	salesReturn := &SalesReturn{Lines: lines}
	salesReturn.RecalculateTotals()
	assertReturnDecimal(t, "total amount", salesReturn.TotalAmount, decimal.Zero)
	assertReturnDecimal(t, "base total amount", salesReturn.BaseTotalAmount, decimal.Zero)
	assertReturnDecimal(t, "cost amount", salesReturn.CostAmount, returnDec("240"))
}

func TestBuildReturnLinesZeroPriceZeroCost(t *testing.T) {
	shipment := newReturnTestShipment()
	shipmentLine := shipment.Lines[0]
	shipmentLine.Amount = decimal.Zero
	shipmentLine.TaxAmount = decimal.Zero
	shipmentLine.GrossAmount = decimal.Zero
	shipmentLine.BaseAmount = decimal.Zero
	shipmentLine.BaseTaxAmount = decimal.Zero
	shipmentLine.BaseGrossAmount = decimal.Zero
	shipmentLine.UnitCost = decimal.Zero
	shipmentLine.CostAmount = decimal.Zero

	lines, err := BuildReturnLines(shipment, []ReturnLineInput{
		{ShipmentLineID: shipmentLine.ID, Quantity: returnDec("4")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}

	line := lines[0]
	assertReturnDecimal(t, "amount", line.Amount, decimal.Zero)
	assertReturnDecimal(t, "tax amount", line.TaxAmount, decimal.Zero)
	assertReturnDecimal(t, "gross amount", line.GrossAmount, decimal.Zero)
	assertReturnDecimal(t, "base amount", line.BaseAmount, decimal.Zero)
	assertReturnDecimal(t, "base tax amount", line.BaseTaxAmount, decimal.Zero)
	assertReturnDecimal(t, "base gross amount", line.BaseGrossAmount, decimal.Zero)
	assertReturnDecimal(t, "unit cost", line.UnitCost, decimal.Zero)
	assertReturnDecimal(t, "cost amount", line.CostAmount, decimal.Zero)

	salesReturn := &SalesReturn{Lines: lines}
	salesReturn.RecalculateTotals()
	assertReturnDecimal(t, "total amount", salesReturn.TotalAmount, decimal.Zero)
	assertReturnDecimal(t, "base total amount", salesReturn.BaseTotalAmount, decimal.Zero)
	assertReturnDecimal(t, "cost amount", salesReturn.CostAmount, decimal.Zero)
}

func TestBuildReturnLinesUnknownShipmentLine(t *testing.T) {
	_, err := BuildReturnLines(newReturnTestShipment(), []ReturnLineInput{
		{ShipmentLineID: "other-shipment-line", Quantity: returnDec("1")},
	})
	if !errors.Is(err, ErrShipmentLineNotFound) {
		t.Fatalf("error = %v, want %v", err, ErrShipmentLineNotFound)
	}
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
			wantErr:  ErrShipmentLineNotFound,
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

	if !salesReturn.NetAmount.Add(salesReturn.TaxAmount).Equal(salesReturn.TotalAmount) {
		t.Fatalf("header net + tax = %s, want total %s",
			salesReturn.NetAmount.Add(salesReturn.TaxAmount), salesReturn.TotalAmount)
	}
	if !salesReturn.BaseNetAmount.Add(salesReturn.BaseTaxAmount).Equal(salesReturn.BaseTotalAmount) {
		t.Fatalf("header base net + base tax = %s, want base total %s",
			salesReturn.BaseNetAmount.Add(salesReturn.BaseTaxAmount), salesReturn.BaseTotalAmount)
	}
}

func newReturnRoundingShipment() *Shipment {
	return &Shipment{
		ID: "shipment-rounding",
		Lines: []*ShipmentLine{
			{
				ID:              "shipment-line-rounding",
				TenantID:        "tenant-1",
				OrderLineID:     "order-line-rounding",
				MaterialID:      "material-rounding",
				MaterialName:    "Material Rounding",
				Quantity:        returnDec("3"),
				UnitPrice:       returnDec("33.333333"),
				TaxRateID:       "tax-rate-1",
				TaxRate:         returnDec("0.13"),
				Amount:          returnDec("100"),
				TaxAmount:       returnDec("13"),
				GrossAmount:     returnDec("113"),
				BaseAmount:      returnDec("720"),
				BaseTaxAmount:   returnDec("93.6"),
				BaseGrossAmount: returnDec("813.6"),
				UnitCost:        returnDec("20"),
				CostAmount:      returnDec("60"),
			},
		},
	}
}

func assertReturnLineBalanced(t *testing.T, line *SalesReturnLine) {
	t.Helper()
	if !line.Amount.Add(line.TaxAmount).Equal(line.GrossAmount) {
		t.Fatalf("line net + tax = %s, want gross %s",
			line.Amount.Add(line.TaxAmount), line.GrossAmount)
	}
	if !line.BaseAmount.Add(line.BaseTaxAmount).Equal(line.BaseGrossAmount) {
		t.Fatalf("line base net + base tax = %s, want base gross %s",
			line.BaseAmount.Add(line.BaseTaxAmount), line.BaseGrossAmount)
	}
}

func TestBuildReturnLinesPartialBalances(t *testing.T) {
	lines, err := BuildReturnLines(newReturnRoundingShipment(), []ReturnLineInput{
		{ShipmentLineID: "shipment-line-rounding", Quantity: returnDec("1")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}

	line := lines[0]
	assertReturnDecimal(t, "amount", line.Amount, returnDec("33.3333"))
	assertReturnDecimal(t, "tax amount", line.TaxAmount, returnDec("4.3334"))
	assertReturnDecimal(t, "gross amount", line.GrossAmount, returnDec("37.6667"))
	assertReturnDecimal(t, "base amount", line.BaseAmount, returnDec("240"))
	assertReturnDecimal(t, "base tax amount", line.BaseTaxAmount, returnDec("31.2"))
	assertReturnDecimal(t, "base gross amount", line.BaseGrossAmount, returnDec("271.2"))
	assertReturnDecimal(t, "cost amount", line.CostAmount, returnDec("20"))
	assertReturnLineBalanced(t, line)

	salesReturn := &SalesReturn{Lines: lines}
	salesReturn.RecalculateTotals()
	assertReturnDecimal(t, "net amount", salesReturn.NetAmount, returnDec("33.3333"))
	assertReturnDecimal(t, "tax amount", salesReturn.TaxAmount, returnDec("4.3334"))
	assertReturnDecimal(t, "total amount", salesReturn.TotalAmount, returnDec("37.6667"))
	assertReturnDecimal(t, "base net amount", salesReturn.BaseNetAmount, returnDec("240"))
	assertReturnDecimal(t, "base tax amount", salesReturn.BaseTaxAmount, returnDec("31.2"))
	assertReturnDecimal(t, "base total amount", salesReturn.BaseTotalAmount, returnDec("271.2"))
	if !salesReturn.NetAmount.Add(salesReturn.TaxAmount).Equal(salesReturn.TotalAmount) {
		t.Fatalf("header net + tax = %s, want total %s",
			salesReturn.NetAmount.Add(salesReturn.TaxAmount), salesReturn.TotalAmount)
	}
	if !salesReturn.BaseNetAmount.Add(salesReturn.BaseTaxAmount).Equal(salesReturn.BaseTotalAmount) {
		t.Fatalf("header base net + base tax = %s, want base total %s",
			salesReturn.BaseNetAmount.Add(salesReturn.BaseTaxAmount), salesReturn.BaseTotalAmount)
	}
}

func TestBuildReturnLinesSequentialReturnsNoResidual(t *testing.T) {
	shipment := newReturnRoundingShipment()
	shipmentLine := shipment.Lines[0]

	totalNet := decimal.Zero
	totalTax := decimal.Zero
	totalGross := decimal.Zero
	totalBaseNet := decimal.Zero
	totalBaseTax := decimal.Zero
	totalBaseGross := decimal.Zero
	totalCost := decimal.Zero

	for i := 0; i < 3; i++ {
		lines, err := BuildReturnLines(shipment, []ReturnLineInput{
			{ShipmentLineID: shipmentLine.ID, Quantity: returnDec("1")},
		})
		if err != nil {
			t.Fatalf("return %d: %v", i+1, err)
		}
		line := lines[0]
		assertReturnLineBalanced(t, line)

		salesReturn := &SalesReturn{Lines: lines}
		salesReturn.RecalculateTotals()
		if !salesReturn.NetAmount.Add(salesReturn.TaxAmount).Equal(salesReturn.TotalAmount) {
			t.Fatalf("return %d header net + tax = %s, want total %s",
				i+1, salesReturn.NetAmount.Add(salesReturn.TaxAmount), salesReturn.TotalAmount)
		}

		totalNet = totalNet.Add(line.Amount)
		totalTax = totalTax.Add(line.TaxAmount)
		totalGross = totalGross.Add(line.GrossAmount)
		totalBaseNet = totalBaseNet.Add(line.BaseAmount)
		totalBaseTax = totalBaseTax.Add(line.BaseTaxAmount)
		totalBaseGross = totalBaseGross.Add(line.BaseGrossAmount)
		totalCost = totalCost.Add(line.CostAmount)

		shipmentLine.ReturnedQty = shipmentLine.ReturnedQty.Add(returnDec("1"))
	}

	assertReturnDecimal(t, "total net", totalNet, shipmentLine.Amount)
	assertReturnDecimal(t, "total tax", totalTax, shipmentLine.TaxAmount)
	assertReturnDecimal(t, "total gross", totalGross, shipmentLine.GrossAmount)
	assertReturnDecimal(t, "total base net", totalBaseNet, shipmentLine.BaseAmount)
	assertReturnDecimal(t, "total base tax", totalBaseTax, shipmentLine.BaseTaxAmount)
	assertReturnDecimal(t, "total base gross", totalBaseGross, shipmentLine.BaseGrossAmount)
	assertReturnDecimal(t, "total cost", totalCost, shipmentLine.CostAmount)
	assertReturnDecimal(t, "returned quantity", shipmentLine.ReturnedQty, returnDec("3"))

	_, err := BuildReturnLines(shipment, []ReturnLineInput{
		{ShipmentLineID: shipmentLine.ID, Quantity: returnDec("1")},
	})
	if !errors.Is(err, ErrOverReturn) {
		t.Fatalf("return after full return error = %v, want %v", err, ErrOverReturn)
	}
}
