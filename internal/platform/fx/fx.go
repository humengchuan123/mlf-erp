package fx

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const BaseCurrency = "CNY"

var (
	ErrInvalidRate  = errors.New("invalid exchange rate")
	ErrRateNotFound = errors.New("exchange rate not found")
)

type RateReader interface {
	Latest(ctx context.Context, tenantID, currency string, date time.Time) (decimal.Decimal, error)
}

func Normalize(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

func IsBase(currency string) bool {
	return Normalize(currency) == BaseCurrency
}

func Convert(amount, rate decimal.Decimal) (decimal.Decimal, error) {
	if rate.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, ErrInvalidRate
	}
	return amount.Mul(rate).Round(4), nil
}

func Diff(amount, paymentRate, invoiceRate decimal.Decimal) decimal.Decimal {
	return amount.Mul(paymentRate.Sub(invoiceRate)).Round(4)
}

func Resolve(
	ctx context.Context,
	reader RateReader,
	tenantID, currency string,
	date time.Time,
	override decimal.Decimal,
) (decimal.Decimal, error) {
	currency = Normalize(currency)
	if currency == "" || IsBase(currency) {
		return decimal.NewFromInt(1), nil
	}
	if override.IsNegative() {
		return decimal.Zero, ErrInvalidRate
	}
	if override.IsPositive() {
		return override, nil
	}
	if reader == nil {
		return decimal.Zero, ErrRateNotFound
	}
	rate, err := reader.Latest(ctx, tenantID, currency, date)
	if err != nil {
		return decimal.Zero, err
	}
	if !rate.IsPositive() {
		return decimal.Zero, ErrInvalidRate
	}
	return rate, nil
}
