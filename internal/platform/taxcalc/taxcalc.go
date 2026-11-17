package taxcalc

import (
	"errors"

	"github.com/shopspring/decimal"
)

var ErrInvalidInput = errors.New("invalid tax calculation input")

type Result struct {
	Net   decimal.Decimal
	Tax   decimal.Decimal
	Gross decimal.Decimal
}

// Calculate splits an entered line amount into net and tax.
// When priceIncludesTax is true, unitPrice is treated as tax-inclusive;
// otherwise it is treated as tax-exclusive. Values are rounded to 4 decimals.
func Calculate(quantity, unitPrice, rate decimal.Decimal, priceIncludesTax bool) (Result, error) {
	if quantity.IsNegative() || unitPrice.IsNegative() || rate.IsNegative() ||
		rate.GreaterThanOrEqual(decimal.NewFromInt(1)) {
		return Result{}, ErrInvalidInput
	}

	entered := quantity.Mul(unitPrice).Round(4)
	if priceIncludesTax {
		net := entered.Div(decimal.NewFromInt(1).Add(rate)).Round(4)
		tax := entered.Sub(net).Round(4)
		return Result{Net: net, Tax: tax, Gross: entered}, nil
	}

	tax := entered.Mul(rate).Round(4)
	return Result{Net: entered, Tax: tax, Gross: entered.Add(tax).Round(4)}, nil
}
