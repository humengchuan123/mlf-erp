package postgres

import (
	"errors"
	"testing"

	gldomain "github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
	"github.com/shopspring/decimal"
)

func TestCreditNoteRefundPostingLines(t *testing.T) {
	t.Run("customer refund with fx loss", func(t *testing.T) {
		lines := mustRefundPostingLines(t, CreditNoteRefundPosting{
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
		lines := mustRefundPostingLines(t, CreditNoteRefundPosting{
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

	t.Run("customer refund with fx gain", func(t *testing.T) {
		lines := mustRefundPostingLines(t, CreditNoteRefundPosting{
			CreditType:     "sales_return",
			PartnerID:      "p1",
			BaseAmount:     decimal.RequireFromString("730"),
			CashBaseAmount: decimal.RequireFromString("720"),
			FXAdjustment:   decimal.RequireFromString("-10"),
		})
		if len(lines) != 3 {
			t.Fatalf("line count = %d, want 3", len(lines))
		}
		assertPostingLine(t, findPostingLine(t, lines, "ar"), "ar", decimal.RequireFromString("730"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "bank"), "bank", decimal.Zero, decimal.RequireFromString("720"))
		assertPostingLine(t, findPostingLine(t, lines, "fx_gain"), "fx_gain", decimal.Zero, decimal.RequireFromString("10"))
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("supplier refund with fx loss", func(t *testing.T) {
		lines := mustRefundPostingLines(t, CreditNoteRefundPosting{
			CreditType:     "purchase_return",
			PartnerID:      "p1",
			BaseAmount:     decimal.RequireFromString("730"),
			CashBaseAmount: decimal.RequireFromString("720"),
			FXAdjustment:   decimal.RequireFromString("-10"),
		})
		if len(lines) != 3 {
			t.Fatalf("line count = %d, want 3", len(lines))
		}
		assertPostingLine(t, findPostingLine(t, lines, "bank"), "bank", decimal.RequireFromString("720"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "ap"), "ap", decimal.Zero, decimal.RequireFromString("730"))
		assertPostingLine(t, findPostingLine(t, lines, "fx_loss"), "fx_loss", decimal.RequireFromString("10"), decimal.Zero)
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("zero base customer refund", func(t *testing.T) {
		lines := mustRefundPostingLines(t, CreditNoteRefundPosting{
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
		lines := mustRefundPostingLines(t, CreditNoteRefundPosting{
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

	t.Run("cash method uses cash account", func(t *testing.T) {
		lines := mustRefundPostingLines(t, CreditNoteRefundPosting{
			CreditType:     "sales_return",
			PartnerID:      "p1",
			Method:         "cash",
			BaseAmount:     decimal.RequireFromString("720"),
			CashBaseAmount: decimal.RequireFromString("730"),
			FXAdjustment:   decimal.RequireFromString("10"),
		})
		if len(lines) != 3 {
			t.Fatalf("line count = %d, want 3", len(lines))
		}
		if hasPostingLine(lines, "bank") {
			t.Fatal("cash refund must not post a bank line")
		}
		assertPostingLine(t, findPostingLine(t, lines, "ar"), "ar", decimal.RequireFromString("720"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "cash"), "cash", decimal.Zero, decimal.RequireFromString("730"))
		assertPostingLine(t, findPostingLine(t, lines, "fx_loss"), "fx_loss", decimal.RequireFromString("10"), decimal.Zero)
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("partner ids present on ar ap and fx lines", func(t *testing.T) {
		sales := mustRefundPostingLines(t, CreditNoteRefundPosting{
			CreditType:     "sales_return",
			PartnerID:      "p1",
			BaseAmount:     decimal.RequireFromString("720"),
			CashBaseAmount: decimal.RequireFromString("730"),
			FXAdjustment:   decimal.RequireFromString("10"),
		})
		for _, key := range []string{"ar", "fx_loss"} {
			if partnerID := findPostingLine(t, sales, key).PartnerID; partnerID != "p1" {
				t.Fatalf("%s partner = %q, want p1", key, partnerID)
			}
		}

		purchase := mustRefundPostingLines(t, CreditNoteRefundPosting{
			CreditType:     "purchase_return",
			PartnerID:      "p1",
			BaseAmount:     decimal.RequireFromString("720"),
			CashBaseAmount: decimal.RequireFromString("730"),
			FXAdjustment:   decimal.RequireFromString("10"),
		})
		for _, key := range []string{"ap", "fx_gain"} {
			if partnerID := findPostingLine(t, purchase, key).PartnerID; partnerID != "p1" {
				t.Fatalf("%s partner = %q, want p1", key, partnerID)
			}
		}
	})

	t.Run("zero fx adjustment omits fx line", func(t *testing.T) {
		lines := mustRefundPostingLines(t, CreditNoteRefundPosting{
			CreditType:     "sales_return",
			PartnerID:      "p1",
			BaseAmount:     decimal.RequireFromString("720"),
			CashBaseAmount: decimal.RequireFromString("720"),
			FXAdjustment:   decimal.Zero,
		})
		if len(lines) != 2 {
			t.Fatalf("line count = %d, want 2", len(lines))
		}
		if hasPostingLine(lines, "fx_loss") || hasPostingLine(lines, "fx_gain") {
			t.Fatal("zero fx adjustment must not post an fx line")
		}
		assertPostingLine(t, findPostingLine(t, lines, "ar"), "ar", decimal.RequireFromString("720"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "bank"), "bank", decimal.Zero, decimal.RequireFromString("720"))
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("invalid credit type", func(t *testing.T) {
		assertRefundPostingError(t, CreditNoteRefundPosting{
			CreditType:     "other",
			BaseAmount:     decimal.RequireFromString("720"),
			CashBaseAmount: decimal.RequireFromString("720"),
		})
	})

	t.Run("invalid method", func(t *testing.T) {
		assertRefundPostingError(t, CreditNoteRefundPosting{
			CreditType:     "sales_return",
			Method:         "other",
			BaseAmount:     decimal.RequireFromString("720"),
			CashBaseAmount: decimal.RequireFromString("720"),
		})
	})

	t.Run("mismatched fx adjustment", func(t *testing.T) {
		assertRefundPostingError(t, CreditNoteRefundPosting{
			CreditType:     "sales_return",
			BaseAmount:     decimal.RequireFromString("720"),
			CashBaseAmount: decimal.RequireFromString("730"),
			FXAdjustment:   decimal.RequireFromString("9"),
		})
	})

	t.Run("invalid amounts", func(t *testing.T) {
		assertRefundPostingError(t, CreditNoteRefundPosting{
			CreditType:     "sales_return",
			BaseAmount:     decimal.RequireFromString("-10"),
			CashBaseAmount: decimal.RequireFromString("100"),
			FXAdjustment:   decimal.RequireFromString("110"),
		})
		assertRefundPostingError(t, CreditNoteRefundPosting{
			CreditType:     "sales_return",
			BaseAmount:     decimal.Zero,
			CashBaseAmount: decimal.Zero,
		})
	})
}

func mustRefundPostingLines(t *testing.T, input CreditNoteRefundPosting) []postingLine {
	t.Helper()
	lines, err := creditNoteRefundPostingLines(input)
	if err != nil {
		t.Fatalf("creditNoteRefundPostingLines() error = %v", err)
	}
	return lines
}

func assertRefundPostingError(t *testing.T, input CreditNoteRefundPosting) {
	t.Helper()
	lines, err := creditNoteRefundPostingLines(input)
	if !errors.Is(err, gldomain.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if lines != nil {
		t.Fatalf("lines = %v, want nil", lines)
	}
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
