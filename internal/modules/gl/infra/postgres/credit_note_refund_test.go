package postgres

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestCreditNoteRefundPostingLines(t *testing.T) {
	t.Run("customer refund with fx loss", func(t *testing.T) {
		lines := creditNoteRefundPostingLines(CreditNoteRefundPosting{
			CreditType:     "sales_return",
			PartnerID:      "p1",
			BaseAmount:     decimal.RequireFromString("720"),
			CashBaseAmount: decimal.RequireFromString("730"),
			FXAdjustment:   decimal.RequireFromString("10"),
		})
		if len(lines) != 3 {
			t.Fatalf("line count = %d, want 3", len(lines))
		}
		assertPostingLine(t, findPostingLine(t, lines, "ar"), "ar", decimal.RequireFromString("720"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "bank"), "bank", decimal.Zero, decimal.RequireFromString("730"))
		assertPostingLine(t, findPostingLine(t, lines, "fx_loss"), "fx_loss", decimal.RequireFromString("10"), decimal.Zero)
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("supplier refund with fx gain", func(t *testing.T) {
		lines := creditNoteRefundPostingLines(CreditNoteRefundPosting{
			CreditType:     "purchase_return",
			PartnerID:      "p1",
			BaseAmount:     decimal.RequireFromString("720"),
			CashBaseAmount: decimal.RequireFromString("730"),
			FXAdjustment:   decimal.RequireFromString("10"),
		})
		if len(lines) != 3 {
			t.Fatalf("line count = %d, want 3", len(lines))
		}
		assertPostingLine(t, findPostingLine(t, lines, "bank"), "bank", decimal.RequireFromString("730"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "ap"), "ap", decimal.Zero, decimal.RequireFromString("720"))
		assertPostingLine(t, findPostingLine(t, lines, "fx_gain"), "fx_gain", decimal.Zero, decimal.RequireFromString("10"))
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("zero base customer refund", func(t *testing.T) {
		lines := creditNoteRefundPostingLines(CreditNoteRefundPosting{
			CreditType:     "sales_return",
			PartnerID:      "p1",
			BaseAmount:     decimal.Zero,
			CashBaseAmount: decimal.RequireFromString("100"),
			FXAdjustment:   decimal.RequireFromString("100"),
		})
		if len(lines) != 2 {
			t.Fatalf("line count = %d, want 2", len(lines))
		}
		if hasPostingLine(lines, "ar") {
			t.Fatal("zero-base customer refund must not post an AR line")
		}
		assertPostingLine(t, findPostingLine(t, lines, "bank"), "bank", decimal.Zero, decimal.RequireFromString("100"))
		assertPostingLine(t, findPostingLine(t, lines, "fx_loss"), "fx_loss", decimal.RequireFromString("100"), decimal.Zero)
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("zero base supplier refund", func(t *testing.T) {
		lines := creditNoteRefundPostingLines(CreditNoteRefundPosting{
			CreditType:     "purchase_return",
			PartnerID:      "p1",
			BaseAmount:     decimal.Zero,
			CashBaseAmount: decimal.RequireFromString("100"),
			FXAdjustment:   decimal.RequireFromString("100"),
		})
		if len(lines) != 2 {
			t.Fatalf("line count = %d, want 2", len(lines))
		}
		if hasPostingLine(lines, "ap") {
			t.Fatal("zero-base supplier refund must not post an AP line")
		}
		assertPostingLine(t, findPostingLine(t, lines, "bank"), "bank", decimal.RequireFromString("100"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "fx_gain"), "fx_gain", decimal.Zero, decimal.RequireFromString("100"))
		assertPostingLinesBalanced(t, lines)
	})
}

func findPostingLine(t *testing.T, lines []postingLine, accountKey string) postingLine {
	t.Helper()
	for _, line := range lines {
		if line.AccountKey == accountKey {
			return line
		}
	}
	t.Fatalf("missing posting line %q", accountKey)
	return postingLine{}
}

func hasPostingLine(lines []postingLine, accountKey string) bool {
	for _, line := range lines {
		if line.AccountKey == accountKey {
			return true
		}
	}
	return false
}
