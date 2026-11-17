package postgres

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestFXRevaluationLinesReverse(t *testing.T) {
	items := []fxRevaluationItem{
		{InvoiceID: "ar-1", InvoiceType: "receivable", PartnerID: "customer", Adjustment: decimal.RequireFromString("10")},
		{InvoiceID: "ap-1", InvoiceType: "payable", PartnerID: "supplier", Adjustment: decimal.RequireFromString("6")},
	}

	original := fxRevaluationLines(items, false)
	reversed := fxRevaluationLines(items, true)

	if len(original) != 4 || len(reversed) != 4 {
		t.Fatalf("line counts = %d/%d, want 4/4", len(original), len(reversed))
	}
	for i := range original {
		if !original[i].Debit.Equal(reversed[i].Credit) || !original[i].Credit.Equal(reversed[i].Debit) {
			t.Fatalf("line %d not reversed: original debit=%s credit=%s, reversed debit=%s credit=%s",
				i, original[i].Debit, original[i].Credit, reversed[i].Debit, reversed[i].Credit)
		}
	}
	if original[0].AccountKey != "ar" || !original[0].Debit.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("receivable original = %+v, want ar debit 10", original[0])
	}
	if reversed[0].AccountKey != "ar" || !reversed[0].Credit.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("receivable reversal = %+v, want ar credit 10", reversed[0])
	}
	if original[2].AccountKey != "fx_loss" || !original[2].Debit.Equal(decimal.RequireFromString("6")) {
		t.Fatalf("payable original = %+v, want fx_loss debit 6", original[2])
	}
	if reversed[2].AccountKey != "fx_loss" || !reversed[2].Credit.Equal(decimal.RequireFromString("6")) {
		t.Fatalf("payable reversal = %+v, want fx_loss credit 6", reversed[2])
	}
}

func TestFXRevaluationLinesSkipsZero(t *testing.T) {
	lines := fxRevaluationLines([]fxRevaluationItem{
		{InvoiceID: "ar-1", InvoiceType: "receivable", Adjustment: decimal.Zero},
	}, false)
	if len(lines) != 0 {
		t.Fatalf("zero adjustment produced %d lines", len(lines))
	}
}
