package domain

import "github.com/shopspring/decimal"

// AllocateBase converts an original-currency amount into base currency in
// proportion to the credit note's remaining amount and base amount.
//
// Callers must validate that amount does not exceed remainingAmount. As a
// defensive clamp, amounts greater than or equal to remainingAmount return the
// full remaining base amount (rounded to 4 decimal places).
func AllocateBase(remainingAmount, remainingBase, amount decimal.Decimal) decimal.Decimal {
	if !remainingAmount.IsPositive() || !amount.IsPositive() {
		return decimal.Zero
	}
	if amount.GreaterThanOrEqual(remainingAmount) {
		return remainingBase.Round(4)
	}
	return remainingBase.Mul(amount).DivRound(remainingAmount, 4)
}

// CashBase computes the base-currency cash amount for a refund using the
// refund date exchange rate, rounded to 4 decimal places.
func CashBase(amount, rate decimal.Decimal) decimal.Decimal {
	return amount.Mul(rate).Round(4)
}

// FXAdjustment computes the foreign-exchange gain or loss as cash base amount
// minus the credit-note base amount consumed by the refund, rounded to 4
// decimal places.
func FXAdjustment(cashBase, creditBase decimal.Decimal) decimal.Decimal {
	return cashBase.Sub(creditBase).Round(4)
}

// CreditNoteStatus derives the credit note status from its remaining original
// and base amounts. A note is applied as soon as the original amount is fully
// used, even when a base-currency residual remains. The residual is a display
// and FX amount, not an actionable balance: allocation and refund both require
// a positive remaining original amount.
func CreditNoteStatus(remainingAmount, remainingBase decimal.Decimal) string {
	if remainingAmount.IsZero() {
		return CreditNoteStatusApplied
	}
	return CreditNoteStatusOpen
}
