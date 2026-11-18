package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestCreditNoteRemaining(t *testing.T) {
	note := &CreditNote{
		Amount:        decimal.RequireFromString("100.00"),
		AppliedAmount: decimal.RequireFromString("30.25"),
	}
	if got := note.Remaining(); !got.Equal(decimal.RequireFromString("69.75")) {
		t.Fatalf("remaining = %s, want 69.75", got)
	}
}

func TestCreditNoteRemainingFullyApplied(t *testing.T) {
	note := &CreditNote{
		Amount:        decimal.RequireFromString("100"),
		AppliedAmount: decimal.RequireFromString("100"),
	}
	if got := note.Remaining(); !got.IsZero() {
		t.Fatalf("remaining = %s, want 0", got)
	}
}

func TestCreditNoteStatusConstants(t *testing.T) {
	if CreditNoteStatusOpen != "open" {
		t.Fatalf("open status = %q, want open", CreditNoteStatusOpen)
	}
	if CreditNoteStatusApplied != "applied" {
		t.Fatalf("applied status = %q, want applied", CreditNoteStatusApplied)
	}
	if CreditNoteTypeSalesReturn != "sales_return" {
		t.Fatalf("sales return type = %q, want sales_return", CreditNoteTypeSalesReturn)
	}
}

func TestCreditNoteRemainingRounds(t *testing.T) {
	note := &CreditNote{
		Amount:        decimal.RequireFromString("10.00005"),
		AppliedAmount: decimal.Zero,
	}
	if got := note.Remaining(); !got.Equal(decimal.RequireFromString("10.0001")) {
		t.Fatalf("remaining = %s, want 10.0001", got)
	}
}
