package app

import (
	"context"
	"time"

	"github.com/mlf-erp/mlf-erp/internal/modules/finance/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/fx"
	"github.com/shopspring/decimal"
)

type ExchangeRateService struct {
	rates domain.ExchangeRateRepository
}

func NewExchangeRateService(rates domain.ExchangeRateRepository) *ExchangeRateService {
	return &ExchangeRateService{rates: rates}
}

func (s *ExchangeRateService) List(
	ctx context.Context,
	currency string,
	startDate, endDate time.Time,
	page, pageSize int,
) ([]*domain.ExchangeRate, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.rates.List(ctx, tenantID, fx.Normalize(currency), startDate, endDate, (page-1)*pageSize, pageSize)
}

func (s *ExchangeRateService) Latest(ctx context.Context, currency string, date time.Time) (decimal.Decimal, error) {
	tenantID, _, _ := tenantActor(ctx)
	if fx.IsBase(currency) {
		return decimal.NewFromInt(1), nil
	}
	if date.IsZero() {
		date = time.Now()
	}
	return s.rates.Latest(ctx, tenantID, fx.Normalize(currency), date)
}

func (s *ExchangeRateService) Upsert(ctx context.Context, input domain.ExchangeRateInput) (*domain.ExchangeRate, error) {
	tenantID, _, userID := tenantActor(ctx)
	normalized, err := normalizeExchangeRateInput(input)
	if err != nil {
		return nil, err
	}
	return s.rates.Upsert(ctx, tenantID, userID, normalized)
}

func (s *ExchangeRateService) Update(ctx context.Context, id string, input domain.ExchangeRateInput) (*domain.ExchangeRate, error) {
	tenantID, _, userID := tenantActor(ctx)
	normalized, err := normalizeExchangeRateInput(input)
	if err != nil {
		return nil, err
	}
	return s.rates.Update(ctx, tenantID, id, userID, normalized)
}

func normalizeExchangeRateInput(input domain.ExchangeRateInput) (domain.ExchangeRateInput, error) {
	currency := fx.Normalize(input.CurrencyCode)
	if currency == "" || fx.IsBase(currency) || !input.Rate.IsPositive() || input.RateDate.IsZero() {
		return domain.ExchangeRateInput{}, domain.ErrInvalidInput
	}
	input.CurrencyCode = currency
	return input, nil
}
