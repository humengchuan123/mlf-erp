package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type CurrencyService struct {
	repo domain.CrudRepository[domain.Currency]
}

func NewCurrencyService(repo domain.CrudRepository[domain.Currency]) *CurrencyService {
	return &CurrencyService{repo: repo}
}

type CurrencyInput struct {
	Code          string
	Name          string
	Symbol        string
	DecimalPlaces int
}

func (s *CurrencyService) List(ctx context.Context, keyword string, page, pageSize int) ([]domain.Currency, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.repo.List(ctx, scope.Filter{DataScope: "all"}, tenantID, strings.TrimSpace(keyword), (page-1)*pageSize, pageSize)
}

func (s *CurrencyService) Get(ctx context.Context, id string) (domain.Currency, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.repo.FindByID(ctx, scope.Filter{DataScope: "all"}, tenantID, id)
}

func (s *CurrencyService) Create(ctx context.Context, input CurrencyInput) (domain.Currency, error) {
	tenantID, _, _ := tenantActor(ctx)
	code := strings.TrimSpace(input.Code)
	name := strings.TrimSpace(input.Name)
	if code == "" || name == "" || input.DecimalPlaces < 0 || input.DecimalPlaces > 6 {
		return domain.Currency{}, domain.ErrInvalidInput
	}

	currency := domain.Currency{
		ID:            uuid.NewString(),
		TenantID:      tenantID,
		Code:          strings.ToUpper(code),
		Name:          name,
		Symbol:        strings.TrimSpace(input.Symbol),
		DecimalPlaces: input.DecimalPlaces,
		Status:        domain.StatusActive,
	}
	if err := s.repo.Create(ctx, currency); err != nil {
		return domain.Currency{}, err
	}
	return currency, nil
}

func (s *CurrencyService) Update(ctx context.Context, id string, input CurrencyInput) (domain.Currency, error) {
	tenantID, _, _ := tenantActor(ctx)
	filter := scope.Filter{DataScope: "all"}
	currency, err := s.repo.FindByID(ctx, filter, tenantID, id)
	if err != nil {
		return domain.Currency{}, err
	}

	code := strings.TrimSpace(input.Code)
	name := strings.TrimSpace(input.Name)
	if code == "" || name == "" || input.DecimalPlaces < 0 || input.DecimalPlaces > 6 {
		return domain.Currency{}, domain.ErrInvalidInput
	}
	currency.Code = strings.ToUpper(code)
	currency.Name = name
	currency.Symbol = strings.TrimSpace(input.Symbol)
	currency.DecimalPlaces = input.DecimalPlaces

	if err := s.repo.Update(ctx, filter, currency); err != nil {
		return domain.Currency{}, err
	}
	return currency, nil
}

func (s *CurrencyService) Disable(ctx context.Context, id string) error {
	tenantID, _, userID := tenantActor(ctx)
	return s.repo.Disable(ctx, scope.Filter{DataScope: "all"}, tenantID, id, userID)
}

func (s *CurrencyService) Enable(ctx context.Context, id string) error {
	tenantID, _, userID := tenantActor(ctx)
	return s.repo.Enable(ctx, scope.Filter{DataScope: "all"}, tenantID, id, userID)
}
