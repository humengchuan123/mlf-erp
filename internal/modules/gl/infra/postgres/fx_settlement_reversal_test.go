package postgres

import (
	"errors"
	"testing"

	gldomain "github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
	"github.com/shopspring/decimal"
)

func TestFXSettlementPostingLines(t *testing.T) {
	t.Run("receivable positive books ar debit and fx gain credit", func(t *testing.T) {
		lines := mustFXSettlementPostingLines(t, FXSettlementPosting{
			PartnerID:      "p1",
			InvoiceType:    "receivable",
			BaseAdjustment: decimal.RequireFromString("100"),
		})
		if len(lines) != 2 {
			t.Fatalf("line count = %d, want 2", len(lines))
		}
		assertPostingLine(t, findPostingLine(t, lines, "ar"), "ar", decimal.RequireFromString("100"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "fx_gain"), "fx_gain", decimal.Zero, decimal.RequireFromString("100"))
		assertFXPartnerIDs(t, lines, "ar", "fx_gain")
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("receivable negative books fx loss debit and ar credit", func(t *testing.T) {
		lines := mustFXSettlementPostingLines(t, FXSettlementPosting{
			PartnerID:      "p1",
			InvoiceType:    "receivable",
			BaseAdjustment: decimal.RequireFromString("-100"),
		})
		if len(lines) != 2 {
			t.Fatalf("line count = %d, want 2", len(lines))
		}
		assertPostingLine(t, findPostingLine(t, lines, "fx_loss"), "fx_loss", decimal.RequireFromString("100"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "ar"), "ar", decimal.Zero, decimal.RequireFromString("100"))
		assertFXPartnerIDs(t, lines, "fx_loss", "ar")
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("payable positive books fx loss debit and ap credit", func(t *testing.T) {
		lines := mustFXSettlementPostingLines(t, FXSettlementPosting{
			PartnerID:      "p1",
			InvoiceType:    "payable",
			BaseAdjustment: decimal.RequireFromString("100"),
		})
		if len(lines) != 2 {
			t.Fatalf("line count = %d, want 2", len(lines))
		}
		assertPostingLine(t, findPostingLine(t, lines, "fx_loss"), "fx_loss", decimal.RequireFromString("100"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "ap"), "ap", decimal.Zero, decimal.RequireFromString("100"))
		assertFXPartnerIDs(t, lines, "fx_loss", "ap")
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("payable negative books ap debit and fx gain credit", func(t *testing.T) {
		lines := mustFXSettlementPostingLines(t, FXSettlementPosting{
			PartnerID:      "p1",
			InvoiceType:    "payable",
			BaseAdjustment: decimal.RequireFromString("-100"),
		})
		if len(lines) != 2 {
			t.Fatalf("line count = %d, want 2", len(lines))
		}
		assertPostingLine(t, findPostingLine(t, lines, "ap"), "ap", decimal.RequireFromString("100"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "fx_gain"), "fx_gain", decimal.Zero, decimal.RequireFromString("100"))
		assertFXPartnerIDs(t, lines, "ap", "fx_gain")
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("zero adjustment returns no lines", func(t *testing.T) {
		lines := mustFXSettlementPostingLines(t, FXSettlementPosting{
			PartnerID:      "p1",
			InvoiceType:    "receivable",
			BaseAdjustment: decimal.Zero,
		})
		if len(lines) != 0 {
			t.Fatalf("line count = %d, want 0", len(lines))
		}
	})

	t.Run("rounds adjustment to four decimals", func(t *testing.T) {
		lines := mustFXSettlementPostingLines(t, FXSettlementPosting{
			PartnerID:      "p1",
			InvoiceType:    "receivable",
			BaseAdjustment: decimal.RequireFromString("100.00004"),
		})
		if len(lines) != 2 {
			t.Fatalf("line count = %d, want 2", len(lines))
		}
		assertPostingLine(t, findPostingLine(t, lines, "ar"), "ar", decimal.RequireFromString("100"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "fx_gain"), "fx_gain", decimal.Zero, decimal.RequireFromString("100"))
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("rounds positive adjustment to zero and returns no lines", func(t *testing.T) {
		lines := mustFXSettlementPostingLines(t, FXSettlementPosting{
			PartnerID:      "p1",
			InvoiceType:    "receivable",
			BaseAdjustment: decimal.RequireFromString("0.00004"),
		})
		if len(lines) != 0 {
			t.Fatalf("line count = %d, want 0", len(lines))
		}
	})

	t.Run("rounds negative adjustment to zero and returns no lines", func(t *testing.T) {
		lines := mustFXSettlementPostingLines(t, FXSettlementPosting{
			PartnerID:      "p1",
			InvoiceType:    "receivable",
			BaseAdjustment: decimal.RequireFromString("-0.00004"),
		})
		if len(lines) != 0 {
			t.Fatalf("line count = %d, want 0", len(lines))
		}
	})

	t.Run("invalid invoice type returns error", func(t *testing.T) {
		lines, err := fxSettlementPostingLines(FXSettlementPosting{
			PartnerID:      "p1",
			InvoiceType:    "other",
			BaseAdjustment: decimal.RequireFromString("100"),
		})
		if !errors.Is(err, gldomain.ErrInvalidInput) {
			t.Fatalf("error = %v, want ErrInvalidInput", err)
		}
		if lines != nil {
			t.Fatalf("lines = %v, want nil", lines)
		}
	})

	t.Run("invalid invoice type wins over zero adjustment", func(t *testing.T) {
		lines, err := fxSettlementPostingLines(FXSettlementPosting{
			PartnerID:      "p1",
			InvoiceType:    "other",
			BaseAdjustment: decimal.Zero,
		})
		if !errors.Is(err, gldomain.ErrInvalidInput) {
			t.Fatalf("error = %v, want ErrInvalidInput", err)
		}
		if lines != nil {
			t.Fatalf("lines = %v, want nil", lines)
		}
	})
}

func TestFXSettlementReversalLines(t *testing.T) {
	t.Run("receivable positive reversal books fx loss debit and ar credit", func(t *testing.T) {
		lines := mustFXSettlementReversalLines(t, FXSettlementReversalPosting{
			PartnerID:      "p1",
			InvoiceType:    "receivable",
			BaseAdjustment: decimal.RequireFromString("100"),
		})
		if len(lines) != 2 {
			t.Fatalf("line count = %d, want 2", len(lines))
		}
		assertPostingLine(t, findPostingLine(t, lines, "fx_loss"), "fx_loss", decimal.RequireFromString("100"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "ar"), "ar", decimal.Zero, decimal.RequireFromString("100"))
		assertFXPartnerIDs(t, lines, "fx_loss", "ar")
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("receivable negative reversal books ar debit and fx gain credit", func(t *testing.T) {
		lines := mustFXSettlementReversalLines(t, FXSettlementReversalPosting{
			PartnerID:      "p1",
			InvoiceType:    "receivable",
			BaseAdjustment: decimal.RequireFromString("-100"),
		})
		if len(lines) != 2 {
			t.Fatalf("line count = %d, want 2", len(lines))
		}
		assertPostingLine(t, findPostingLine(t, lines, "ar"), "ar", decimal.RequireFromString("100"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "fx_gain"), "fx_gain", decimal.Zero, decimal.RequireFromString("100"))
		assertFXPartnerIDs(t, lines, "ar", "fx_gain")
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("payable positive reversal books ap debit and fx gain credit", func(t *testing.T) {
		lines := mustFXSettlementReversalLines(t, FXSettlementReversalPosting{
			PartnerID:      "p1",
			InvoiceType:    "payable",
			BaseAdjustment: decimal.RequireFromString("100"),
		})
		if len(lines) != 2 {
			t.Fatalf("line count = %d, want 2", len(lines))
		}
		assertPostingLine(t, findPostingLine(t, lines, "ap"), "ap", decimal.RequireFromString("100"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "fx_gain"), "fx_gain", decimal.Zero, decimal.RequireFromString("100"))
		assertFXPartnerIDs(t, lines, "ap", "fx_gain")
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("payable negative reversal books fx loss debit and ap credit", func(t *testing.T) {
		lines := mustFXSettlementReversalLines(t, FXSettlementReversalPosting{
			PartnerID:      "p1",
			InvoiceType:    "payable",
			BaseAdjustment: decimal.RequireFromString("-100"),
		})
		if len(lines) != 2 {
			t.Fatalf("line count = %d, want 2", len(lines))
		}
		assertPostingLine(t, findPostingLine(t, lines, "fx_loss"), "fx_loss", decimal.RequireFromString("100"), decimal.Zero)
		assertPostingLine(t, findPostingLine(t, lines, "ap"), "ap", decimal.Zero, decimal.RequireFromString("100"))
		assertFXPartnerIDs(t, lines, "fx_loss", "ap")
		assertPostingLinesBalanced(t, lines)
	})

	t.Run("zero adjustment returns no lines", func(t *testing.T) {
		lines := mustFXSettlementReversalLines(t, FXSettlementReversalPosting{
			PartnerID:      "p1",
			InvoiceType:    "receivable",
			BaseAdjustment: decimal.Zero,
		})
		if len(lines) != 0 {
			t.Fatalf("line count = %d, want 0", len(lines))
		}
	})

	t.Run("invalid invoice type returns error", func(t *testing.T) {
		lines, err := fxSettlementReversalPostingLines(FXSettlementReversalPosting{
			PartnerID:      "p1",
			InvoiceType:    "other",
			BaseAdjustment: decimal.RequireFromString("100"),
		})
		if !errors.Is(err, gldomain.ErrInvalidInput) {
			t.Fatalf("error = %v, want ErrInvalidInput", err)
		}
		if lines != nil {
			t.Fatalf("lines = %v, want nil", lines)
		}
	})
}

func mustFXSettlementPostingLines(t *testing.T, input FXSettlementPosting) []postingLine {
	t.Helper()
	lines, err := fxSettlementPostingLines(input)
	if err != nil {
		t.Fatalf("fxSettlementPostingLines() error = %v", err)
	}
	return lines
}

func mustFXSettlementReversalLines(t *testing.T, input FXSettlementReversalPosting) []postingLine {
	t.Helper()
	lines, err := fxSettlementReversalPostingLines(input)
	if err != nil {
		t.Fatalf("fxSettlementReversalPostingLines() error = %v", err)
	}
	return lines
}

func assertFXPartnerIDs(t *testing.T, lines []postingLine, accountKeys ...string) {
	t.Helper()
	for _, key := range accountKeys {
		if partnerID := findPostingLine(t, lines, key).PartnerID; partnerID != "p1" {
			t.Fatalf("%s partner = %q, want p1", key, partnerID)
		}
	}
}
