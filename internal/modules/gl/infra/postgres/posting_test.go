package postgres

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestPurchaseReturnPostingLinesNegativeTax(t *testing.T) {
	lines := purchaseReturnPostingLines(PurchaseReturnPosting{
		SupplierID:      "supplier-1",
		BaseGrossAmount: decimal.RequireFromString("99.9999"),
		BaseNetAmount:   decimal.RequireFromString("100.0000"),
		BaseTaxAmount:   decimal.RequireFromString("-0.0001"),
	})

	if len(lines) != 3 {
		t.Fatalf("line count = %d, want 3", len(lines))
	}
	assertPostingLine(t, lines[0], "ap", decimal.RequireFromString("99.9999"), decimal.Zero)
	assertPostingLine(t, lines[1], "inventory", decimal.Zero, decimal.RequireFromString("100.0000"))
	assertPostingLine(t, lines[2], "input_vat", decimal.RequireFromString("0.0001"), decimal.Zero)
	assertPostingLinesBalanced(t, lines)
}

func TestPurchaseReturnPostingLinesPositiveTax(t *testing.T) {
	lines := purchaseReturnPostingLines(PurchaseReturnPosting{
		SupplierID:      "supplier-1",
		BaseGrossAmount: decimal.RequireFromString("113"),
		BaseNetAmount:   decimal.RequireFromString("100"),
		BaseTaxAmount:   decimal.RequireFromString("13"),
	})

	if len(lines) != 3 {
		t.Fatalf("line count = %d, want 3", len(lines))
	}
	assertPostingLine(t, lines[0], "ap", decimal.RequireFromString("113"), decimal.Zero)
	assertPostingLine(t, lines[1], "inventory", decimal.Zero, decimal.RequireFromString("100"))
	assertPostingLine(t, lines[2], "input_vat", decimal.Zero, decimal.RequireFromString("13"))
	assertPostingLinesBalanced(t, lines)
}

func TestPurchaseReturnPostingLinesZero(t *testing.T) {
	lines := purchaseReturnPostingLines(PurchaseReturnPosting{})
	if len(lines) != 0 {
		t.Fatalf("line count = %d, want 0", len(lines))
	}
}

func assertPostingLine(t *testing.T, line postingLine, accountKey string, debit, credit decimal.Decimal) {
	t.Helper()
	if line.AccountKey != accountKey {
		t.Fatalf("account key = %q, want %q", line.AccountKey, accountKey)
	}
	if !line.Debit.Equal(debit) {
		t.Fatalf("%s debit = %s, want %s", accountKey, line.Debit, debit)
	}
	if !line.Credit.Equal(credit) {
		t.Fatalf("%s credit = %s, want %s", accountKey, line.Credit, credit)
	}
}

func assertPostingLinesBalanced(t *testing.T, lines []postingLine) {
	t.Helper()
	totalDebit := decimal.Zero
	totalCredit := decimal.Zero
	for _, line := range lines {
		totalDebit = totalDebit.Add(line.Debit)
		totalCredit = totalCredit.Add(line.Credit)
	}
	if !totalDebit.Equal(totalCredit) {
		t.Fatalf("total debit = %s, total credit = %s, want equal", totalDebit, totalCredit)
	}
}
