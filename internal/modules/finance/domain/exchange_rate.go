package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type ExchangeRate struct {
	ID           string
	TenantID     string
	CurrencyCode string
	RateDate     time.Time
	Rate         decimal.Decimal
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ExchangeRateInput struct {
	CurrencyCode string
	RateDate     time.Time
	Rate         decimal.Decimal
}
