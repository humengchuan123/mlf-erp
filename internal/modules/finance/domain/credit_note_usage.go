package domain

import "github.com/shopspring/decimal"

const baseResidualTolerance = "0.0001"

func AllocateBase(remainingAmount, remainingBase, amount decimal.Decimal) decimal.Decimal {
	if !remainingAmount.IsPositive() || !amount.IsPositive() {
		return decimal.Zero
	}
	if amount.GreaterThanOrEqual(remainingAmount) {
		return remainingBase.Round(4)
	}
	return remainingBase.Mul(amount).Div(remainingAmount).Round(4)
}

func CashBase(amount, rate decimal.Decimal) decimal.Decimal {
	return amount.Mul(rate).Round(4)
}

func FXAdjustment(cashBase, creditBase decimal.Decimal) decimal.Decimal {
	return cashBase.Sub(creditBase).Round(4)
}

func CreditNoteStatus(remainingAmount, remainingBase decimal.Decimal) string {
	if remainingAmount.IsZero() &&
		remainingBase.Abs().LessThanOrEqual(decimal.RequireFromString(baseResidualTolerance)) {
		return CreditNoteStatusApplied
	}
	return CreditNoteStatusOpen
}
