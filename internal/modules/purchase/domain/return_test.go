package domain

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func purchaseReturnDec(value string) decimal.Decimal {
	return decimal.RequireFromString(value)
}

func newPurchaseReturnTestReceipt() *Receipt {
	return &Receipt{
		ID: "receipt-1",
		Lines: []*ReceiptLine{
			{
				ID:              "receipt-line-1",
				TenantID:        "tenant-1",
				ReceiptID:       "receipt-1",
				OrderLineID:     "order-line-1",
				MaterialID:      "material-1",
				MaterialName:    "Material A",
				Quantity:        purchaseReturnDec("10"),
				UnitPrice:       purchaseReturnDec("100"),
				TaxRateID:       "tax-rate-1",
				TaxRate:         purchaseReturnDec("0.13"),
				Amount:          purchaseReturnDec("1000"),
				TaxAmount:       purchaseReturnDec("130"),
				GrossAmount:     purchaseReturnDec("1130"),
				BaseAmount:      purchaseReturnDec("7200"),
				BaseTaxAmount:   purchaseReturnDec("936"),
				BaseGrossAmount: purchaseReturnDec("8136"),
			},
			{
				ID:              "receipt-line-2",
				TenantID:        "tenant-1",
				ReceiptID:       "receipt-1",
				OrderLineID:     "order-line-2",
				MaterialID:      "material-2",
				MaterialName:    "Material B",
				Quantity:        purchaseReturnDec("10"),
				UnitPrice:       purchaseReturnDec("50"),
				TaxRate:         decimal.Zero,
				Amount:          purchaseReturnDec("500"),
				TaxAmount:       decimal.Zero,
				GrossAmount:     purchaseReturnDec("500"),
				BaseAmount:      purchaseReturnDec("3600"),
				BaseTaxAmount:   decimal.Zero,
				BaseGrossAmount: purchaseReturnDec("3600"),
			},
		},
	}
}

func assertPurchaseReturnDecimal(t *testing.T, name string, got, want decimal.Decimal) {
	t.Helper()
	if !got.Equal(want) {
		t.Fatalf("%s = %s, want %s", name, got, want)
	}
}

func TestBuildPurchaseReturnLinesPartial(t *testing.T) {
	lines, err := BuildReturnLines(newPurchaseReturnTestReceipt(), []ReturnLineInput{
		{ReceiptLineID: "receipt-line-1", Quantity: purchaseReturnDec("2")},
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
	if line.ReceiptLineID != "receipt-line-1" ||
		line.OrderLineID != "order-line-1" ||
		line.MaterialID != "material-1" ||
		line.MaterialName != "Material A" {
		t.Fatalf("line source fields not copied: %+v", line)
	}
	assertPurchaseReturnDecimal(t, "quantity", line.Quantity, purchaseReturnDec("2"))
	assertPurchaseReturnDecimal(t, "unit price", line.UnitPrice, purchaseReturnDec("100"))
	assertPurchaseReturnDecimal(t, "tax rate", line.TaxRate, purchaseReturnDec("0.13"))
	assertPurchaseReturnDecimal(t, "amount", line.Amount, purchaseReturnDec("200"))
	assertPurchaseReturnDecimal(t, "tax amount", line.TaxAmount, purchaseReturnDec("26"))
	assertPurchaseReturnDecimal(t, "gross amount", line.GrossAmount, purchaseReturnDec("226"))
	assertPurchaseReturnDecimal(t, "base amount", line.BaseAmount, purchaseReturnDec("1440"))
	assertPurchaseReturnDecimal(t, "base tax amount", line.BaseTaxAmount, purchaseReturnDec("187.2"))
	assertPurchaseReturnDecimal(t, "base gross amount", line.BaseGrossAmount, purchaseReturnDec("1627.2"))
	assertPurchaseReturnDecimal(t, "unit cost", line.UnitCost, purchaseReturnDec("720"))
	assertPurchaseReturnDecimal(t, "cost amount", line.CostAmount, purchaseReturnDec("1440"))
}

func TestBuildPurchaseReturnLinesFullReturn(t *testing.T) {
	lines, err := BuildReturnLines(newPurchaseReturnTestReceipt(), []ReturnLineInput{
		{ReceiptLineID: "receipt-line-1", Quantity: purchaseReturnDec("10")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}

	line := lines[0]
	assertPurchaseReturnDecimal(t, "quantity", line.Quantity, purchaseReturnDec("10"))
	assertPurchaseReturnDecimal(t, "amount", line.Amount, purchaseReturnDec("1000"))
	assertPurchaseReturnDecimal(t, "tax amount", line.TaxAmount, purchaseReturnDec("130"))
	assertPurchaseReturnDecimal(t, "gross amount", line.GrossAmount, purchaseReturnDec("1130"))
	assertPurchaseReturnDecimal(t, "base amount", line.BaseAmount, purchaseReturnDec("7200"))
	assertPurchaseReturnDecimal(t, "base tax amount", line.BaseTaxAmount, purchaseReturnDec("936"))
	assertPurchaseReturnDecimal(t, "base gross amount", line.BaseGrossAmount, purchaseReturnDec("8136"))
	assertPurchaseReturnDecimal(t, "unit cost", line.UnitCost, purchaseReturnDec("720"))
	assertPurchaseReturnDecimal(t, "cost amount", line.CostAmount, purchaseReturnDec("7200"))
}

func TestBuildPurchaseReturnLinesZeroTaxAndCost(t *testing.T) {
	receipt := newPurchaseReturnTestReceipt()
	receiptLine := receipt.Lines[1]
	receiptLine.BaseAmount = decimal.Zero
	receiptLine.BaseTaxAmount = decimal.Zero
	receiptLine.BaseGrossAmount = decimal.Zero

	lines, err := BuildReturnLines(receipt, []ReturnLineInput{
		{ReceiptLineID: receiptLine.ID, Quantity: purchaseReturnDec("5")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}

	line := lines[0]
	assertPurchaseReturnDecimal(t, "amount", line.Amount, purchaseReturnDec("250"))
	assertPurchaseReturnDecimal(t, "tax amount", line.TaxAmount, decimal.Zero)
	assertPurchaseReturnDecimal(t, "gross amount", line.GrossAmount, purchaseReturnDec("250"))
	assertPurchaseReturnDecimal(t, "base amount", line.BaseAmount, decimal.Zero)
	assertPurchaseReturnDecimal(t, "base tax amount", line.BaseTaxAmount, decimal.Zero)
	assertPurchaseReturnDecimal(t, "base gross amount", line.BaseGrossAmount, decimal.Zero)
	assertPurchaseReturnDecimal(t, "unit cost", line.UnitCost, decimal.Zero)
	assertPurchaseReturnDecimal(t, "cost amount", line.CostAmount, decimal.Zero)

	purchaseReturn := &PurchaseReturn{Lines: lines}
	purchaseReturn.RecalculateTotals()
	assertPurchaseReturnDecimal(t, "total amount", purchaseReturn.TotalAmount, purchaseReturnDec("250"))
	assertPurchaseReturnDecimal(t, "base total amount", purchaseReturn.BaseTotalAmount, decimal.Zero)
	assertPurchaseReturnDecimal(t, "cost amount", purchaseReturn.CostAmount, decimal.Zero)
}

func TestBuildPurchaseReturnLinesZeroPricePositiveCost(t *testing.T) {
	receipt := newPurchaseReturnTestReceipt()
	receiptLine := receipt.Lines[0]
	receiptLine.Amount = decimal.Zero
	receiptLine.TaxAmount = decimal.Zero
	receiptLine.GrossAmount = decimal.Zero
	receiptLine.BaseTaxAmount = decimal.Zero
	receiptLine.BaseGrossAmount = receiptLine.BaseAmount

	lines, err := BuildReturnLines(receipt, []ReturnLineInput{
		{ReceiptLineID: receiptLine.ID, Quantity: purchaseReturnDec("4")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}

	line := lines[0]
	assertPurchaseReturnDecimal(t, "amount", line.Amount, decimal.Zero)
	assertPurchaseReturnDecimal(t, "tax amount", line.TaxAmount, decimal.Zero)
	assertPurchaseReturnDecimal(t, "gross amount", line.GrossAmount, decimal.Zero)
	assertPurchaseReturnDecimal(t, "base amount", line.BaseAmount, purchaseReturnDec("2880"))
	assertPurchaseReturnDecimal(t, "base tax amount", line.BaseTaxAmount, decimal.Zero)
	assertPurchaseReturnDecimal(t, "base gross amount", line.BaseGrossAmount, purchaseReturnDec("2880"))
	assertPurchaseReturnDecimal(t, "unit cost", line.UnitCost, purchaseReturnDec("720"))
	assertPurchaseReturnDecimal(t, "cost amount", line.CostAmount, purchaseReturnDec("2880"))

	purchaseReturn := &PurchaseReturn{Lines: lines}
	purchaseReturn.RecalculateTotals()
	assertPurchaseReturnDecimal(t, "total amount", purchaseReturn.TotalAmount, decimal.Zero)
	assertPurchaseReturnDecimal(t, "base total amount", purchaseReturn.BaseTotalAmount, purchaseReturnDec("2880"))
	assertPurchaseReturnDecimal(t, "cost amount", purchaseReturn.CostAmount, purchaseReturnDec("2880"))
}

func TestBuildPurchaseReturnLinesUnknownReceiptLine(t *testing.T) {
	_, err := BuildReturnLines(newPurchaseReturnTestReceipt(), []ReturnLineInput{
		{ReceiptLineID: "other-receipt-line", Quantity: purchaseReturnDec("1")},
	})
	if !errors.Is(err, ErrReceiptLineNotFound) {
		t.Fatalf("error = %v, want %v", err, ErrReceiptLineNotFound)
	}
}

func TestBuildPurchaseReturnLinesRemainingQuantity(t *testing.T) {
	receipt := newPurchaseReturnTestReceipt()
	receipt.Lines[0].ReturnedQty = purchaseReturnDec("8")

	lines, err := BuildReturnLines(receipt, []ReturnLineInput{
		{ReceiptLineID: "receipt-line-1", Quantity: purchaseReturnDec("2")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}
	assertPurchaseReturnDecimal(t, "quantity", lines[0].Quantity, purchaseReturnDec("2"))
	assertPurchaseReturnDecimal(t, "amount", lines[0].Amount, purchaseReturnDec("200"))
	assertPurchaseReturnDecimal(t, "base amount", lines[0].BaseAmount, purchaseReturnDec("1440"))
}

func TestBuildPurchaseReturnLinesErrors(t *testing.T) {
	tests := []struct {
		name    string
		receipt func() *Receipt
		inputs  []ReturnLineInput
		wantErr error
	}{
		{
			name:    "nil receipt",
			receipt: func() *Receipt { return nil },
			inputs:  []ReturnLineInput{{ReceiptLineID: "receipt-line-1", Quantity: purchaseReturnDec("1")}},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "no inputs",
			receipt: newPurchaseReturnTestReceipt,
			inputs:  nil,
			wantErr: ErrInvalidInput,
		},
		{
			name:    "empty receipt line id",
			receipt: newPurchaseReturnTestReceipt,
			inputs:  []ReturnLineInput{{Quantity: purchaseReturnDec("1")}},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "unknown receipt line",
			receipt: newPurchaseReturnTestReceipt,
			inputs:  []ReturnLineInput{{ReceiptLineID: "missing", Quantity: purchaseReturnDec("1")}},
			wantErr: ErrReceiptLineNotFound,
		},
		{
			name:    "zero quantity",
			receipt: newPurchaseReturnTestReceipt,
			inputs:  []ReturnLineInput{{ReceiptLineID: "receipt-line-1", Quantity: decimal.Zero}},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "negative quantity",
			receipt: newPurchaseReturnTestReceipt,
			inputs:  []ReturnLineInput{{ReceiptLineID: "receipt-line-1", Quantity: purchaseReturnDec("-1")}},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "duplicate receipt line",
			receipt: newPurchaseReturnTestReceipt,
			inputs: []ReturnLineInput{
				{ReceiptLineID: "receipt-line-1", Quantity: purchaseReturnDec("1")},
				{ReceiptLineID: "receipt-line-1", Quantity: purchaseReturnDec("1")},
			},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "zero receipt quantity",
			receipt: func() *Receipt { r := newPurchaseReturnTestReceipt(); r.Lines[0].Quantity = decimal.Zero; return r },
			inputs:  []ReturnLineInput{{ReceiptLineID: "receipt-line-1", Quantity: purchaseReturnDec("1")}},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "over return",
			receipt: newPurchaseReturnTestReceipt,
			inputs:  []ReturnLineInput{{ReceiptLineID: "receipt-line-1", Quantity: purchaseReturnDec("11")}},
			wantErr: ErrOverReturn,
		},
		{
			name: "over return after previous returns",
			receipt: func() *Receipt {
				r := newPurchaseReturnTestReceipt()
				r.Lines[0].ReturnedQty = purchaseReturnDec("8")
				return r
			},
			inputs:  []ReturnLineInput{{ReceiptLineID: "receipt-line-1", Quantity: purchaseReturnDec("3")}},
			wantErr: ErrOverReturn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildReturnLines(tt.receipt(), tt.inputs)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildPurchaseReturnLinesLineNumbers(t *testing.T) {
	lines, err := BuildReturnLines(newPurchaseReturnTestReceipt(), []ReturnLineInput{
		{ReceiptLineID: "receipt-line-2", Quantity: purchaseReturnDec("5")},
		{ReceiptLineID: "receipt-line-1", Quantity: purchaseReturnDec("2")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}
	if lines[0].LineNo != 1 || lines[1].LineNo != 2 {
		t.Fatalf("line numbers = %d, %d, want 1, 2", lines[0].LineNo, lines[1].LineNo)
	}
	if lines[0].ReceiptLineID != "receipt-line-2" || lines[1].ReceiptLineID != "receipt-line-1" {
		t.Fatalf("line order not preserved: %s, %s", lines[0].ReceiptLineID, lines[1].ReceiptLineID)
	}
}

func TestPurchaseReturnRecalculateTotals(t *testing.T) {
	lines, err := BuildReturnLines(newPurchaseReturnTestReceipt(), []ReturnLineInput{
		{ReceiptLineID: "receipt-line-1", Quantity: purchaseReturnDec("2")},
		{ReceiptLineID: "receipt-line-2", Quantity: purchaseReturnDec("5")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}

	purchaseReturn := &PurchaseReturn{Lines: lines}
	purchaseReturn.RecalculateTotals()

	assertPurchaseReturnDecimal(t, "net amount", purchaseReturn.NetAmount, purchaseReturnDec("450"))
	assertPurchaseReturnDecimal(t, "tax amount", purchaseReturn.TaxAmount, purchaseReturnDec("26"))
	assertPurchaseReturnDecimal(t, "total amount", purchaseReturn.TotalAmount, purchaseReturnDec("476"))
	assertPurchaseReturnDecimal(t, "base net amount", purchaseReturn.BaseNetAmount, purchaseReturnDec("3240"))
	assertPurchaseReturnDecimal(t, "base tax amount", purchaseReturn.BaseTaxAmount, purchaseReturnDec("187.2"))
	assertPurchaseReturnDecimal(t, "base total amount", purchaseReturn.BaseTotalAmount, purchaseReturnDec("3427.2"))
	assertPurchaseReturnDecimal(t, "cost amount", purchaseReturn.CostAmount, purchaseReturnDec("3240"))

	if !purchaseReturn.NetAmount.Add(purchaseReturn.TaxAmount).Equal(purchaseReturn.TotalAmount) {
		t.Fatalf("header net + tax = %s, want total %s",
			purchaseReturn.NetAmount.Add(purchaseReturn.TaxAmount), purchaseReturn.TotalAmount)
	}
	if !purchaseReturn.BaseNetAmount.Add(purchaseReturn.BaseTaxAmount).Equal(purchaseReturn.BaseTotalAmount) {
		t.Fatalf("header base net + base tax = %s, want base total %s",
			purchaseReturn.BaseNetAmount.Add(purchaseReturn.BaseTaxAmount), purchaseReturn.BaseTotalAmount)
	}
}

func newPurchaseReturnRoundingReceipt() *Receipt {
	return &Receipt{
		ID: "receipt-rounding",
		Lines: []*ReceiptLine{
			{
				ID:              "receipt-line-rounding",
				TenantID:        "tenant-1",
				ReceiptID:       "receipt-rounding",
				OrderLineID:     "order-line-rounding",
				MaterialID:      "material-rounding",
				MaterialName:    "Material Rounding",
				Quantity:        purchaseReturnDec("3"),
				UnitPrice:       purchaseReturnDec("33.333333"),
				TaxRateID:       "tax-rate-1",
				TaxRate:         purchaseReturnDec("0.13"),
				Amount:          purchaseReturnDec("100"),
				TaxAmount:       purchaseReturnDec("13"),
				GrossAmount:     purchaseReturnDec("113"),
				BaseAmount:      purchaseReturnDec("720"),
				BaseTaxAmount:   purchaseReturnDec("93.6"),
				BaseGrossAmount: purchaseReturnDec("813.6"),
			},
		},
	}
}

func assertPurchaseReturnLineBalanced(t *testing.T, line *PurchaseReturnLine) {
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

func TestBuildPurchaseReturnLinesPartialBalances(t *testing.T) {
	lines, err := BuildReturnLines(newPurchaseReturnRoundingReceipt(), []ReturnLineInput{
		{ReceiptLineID: "receipt-line-rounding", Quantity: purchaseReturnDec("1")},
	})
	if err != nil {
		t.Fatalf("build return lines: %v", err)
	}

	line := lines[0]
	assertPurchaseReturnDecimal(t, "amount", line.Amount, purchaseReturnDec("33.3333"))
	assertPurchaseReturnDecimal(t, "tax amount", line.TaxAmount, purchaseReturnDec("4.3334"))
	assertPurchaseReturnDecimal(t, "gross amount", line.GrossAmount, purchaseReturnDec("37.6667"))
	assertPurchaseReturnDecimal(t, "base amount", line.BaseAmount, purchaseReturnDec("240"))
	assertPurchaseReturnDecimal(t, "base tax amount", line.BaseTaxAmount, purchaseReturnDec("31.2"))
	assertPurchaseReturnDecimal(t, "base gross amount", line.BaseGrossAmount, purchaseReturnDec("271.2"))
	assertPurchaseReturnDecimal(t, "unit cost", line.UnitCost, purchaseReturnDec("240"))
	assertPurchaseReturnDecimal(t, "cost amount", line.CostAmount, purchaseReturnDec("240"))
	assertPurchaseReturnLineBalanced(t, line)

	purchaseReturn := &PurchaseReturn{Lines: lines}
	purchaseReturn.RecalculateTotals()
	assertPurchaseReturnDecimal(t, "net amount", purchaseReturn.NetAmount, purchaseReturnDec("33.3333"))
	assertPurchaseReturnDecimal(t, "tax amount", purchaseReturn.TaxAmount, purchaseReturnDec("4.3334"))
	assertPurchaseReturnDecimal(t, "total amount", purchaseReturn.TotalAmount, purchaseReturnDec("37.6667"))
	assertPurchaseReturnDecimal(t, "base net amount", purchaseReturn.BaseNetAmount, purchaseReturnDec("240"))
	assertPurchaseReturnDecimal(t, "base tax amount", purchaseReturn.BaseTaxAmount, purchaseReturnDec("31.2"))
	assertPurchaseReturnDecimal(t, "base total amount", purchaseReturn.BaseTotalAmount, purchaseReturnDec("271.2"))
	assertPurchaseReturnDecimal(t, "cost amount", purchaseReturn.CostAmount, purchaseReturnDec("240"))
	if !purchaseReturn.NetAmount.Add(purchaseReturn.TaxAmount).Equal(purchaseReturn.TotalAmount) {
		t.Fatalf("header net + tax = %s, want total %s",
			purchaseReturn.NetAmount.Add(purchaseReturn.TaxAmount), purchaseReturn.TotalAmount)
	}
	if !purchaseReturn.BaseNetAmount.Add(purchaseReturn.BaseTaxAmount).Equal(purchaseReturn.BaseTotalAmount) {
		t.Fatalf("header base net + base tax = %s, want base total %s",
			purchaseReturn.BaseNetAmount.Add(purchaseReturn.BaseTaxAmount), purchaseReturn.BaseTotalAmount)
	}
}

func TestBuildPurchaseReturnLinesSequentialReturnsNoResidual(t *testing.T) {
	receipt := newPurchaseReturnRoundingReceipt()
	receiptLine := receipt.Lines[0]

	totalNet := decimal.Zero
	totalTax := decimal.Zero
	totalGross := decimal.Zero
	totalBaseNet := decimal.Zero
	totalBaseTax := decimal.Zero
	totalBaseGross := decimal.Zero
	totalCost := decimal.Zero

	for i := 0; i < 3; i++ {
		lines, err := BuildReturnLines(receipt, []ReturnLineInput{
			{ReceiptLineID: receiptLine.ID, Quantity: purchaseReturnDec("1")},
		})
		if err != nil {
			t.Fatalf("return %d: %v", i+1, err)
		}
		line := lines[0]
		assertPurchaseReturnLineBalanced(t, line)

		purchaseReturn := &PurchaseReturn{Lines: lines}
		purchaseReturn.RecalculateTotals()
		if !purchaseReturn.NetAmount.Add(purchaseReturn.TaxAmount).Equal(purchaseReturn.TotalAmount) {
			t.Fatalf("return %d header net + tax = %s, want total %s",
				i+1, purchaseReturn.NetAmount.Add(purchaseReturn.TaxAmount), purchaseReturn.TotalAmount)
		}

		totalNet = totalNet.Add(line.Amount)
		totalTax = totalTax.Add(line.TaxAmount)
		totalGross = totalGross.Add(line.GrossAmount)
		totalBaseNet = totalBaseNet.Add(line.BaseAmount)
		totalBaseTax = totalBaseTax.Add(line.BaseTaxAmount)
		totalBaseGross = totalBaseGross.Add(line.BaseGrossAmount)
		totalCost = totalCost.Add(line.CostAmount)

		receiptLine.ReturnedQty = receiptLine.ReturnedQty.Add(purchaseReturnDec("1"))
	}

	assertPurchaseReturnDecimal(t, "total net", totalNet, receiptLine.Amount)
	assertPurchaseReturnDecimal(t, "total tax", totalTax, receiptLine.TaxAmount)
	assertPurchaseReturnDecimal(t, "total gross", totalGross, receiptLine.GrossAmount)
	assertPurchaseReturnDecimal(t, "total base net", totalBaseNet, receiptLine.BaseAmount)
	assertPurchaseReturnDecimal(t, "total base tax", totalBaseTax, receiptLine.BaseTaxAmount)
	assertPurchaseReturnDecimal(t, "total base gross", totalBaseGross, receiptLine.BaseGrossAmount)
	assertPurchaseReturnDecimal(t, "total cost", totalCost, receiptLine.BaseAmount)
	assertPurchaseReturnDecimal(t, "returned quantity", receiptLine.ReturnedQty, purchaseReturnDec("3"))

	_, err := BuildReturnLines(receipt, []ReturnLineInput{
		{ReceiptLineID: receiptLine.ID, Quantity: purchaseReturnDec("1")},
	})
	if !errors.Is(err, ErrOverReturn) {
		t.Fatalf("return after full return error = %v, want %v", err, ErrOverReturn)
	}
}
