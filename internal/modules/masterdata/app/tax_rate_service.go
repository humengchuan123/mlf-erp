package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type TaxRateService struct {
	repo domain.CrudRepository[domain.TaxRate]
}

func NewTaxRateService(repo domain.CrudRepository[domain.TaxRate]) *TaxRateService {
	return &TaxRateService{repo: repo}
}

type TaxRateInput struct {
	Code string
	Name string
	Rate string
	Type string
}

func (s *TaxRateService) List(ctx context.Context, keyword string, page, pageSize int) ([]domain.TaxRate, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.repo.List(ctx, scope.Filter{DataScope: "all"}, tenantID, strings.TrimSpace(keyword), (page-1)*pageSize, pageSize)
}

func (s *TaxRateService) Get(ctx context.Context, id string) (domain.TaxRate, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.repo.FindByID(ctx, scope.Filter{DataScope: "all"}, tenantID, id)
}

func (s *TaxRateService) Create(ctx context.Context, input TaxRateInput) (domain.TaxRate, error) {
	tenantID, _, _ := tenantActor(ctx)
	code := strings.TrimSpace(input.Code)
	name := strings.TrimSpace(input.Name)
	rate, err := decimal.NewFromString(strings.TrimSpace(input.Rate))
	if err != nil || rate.IsNegative() {
		return domain.TaxRate{}, domain.ErrInvalidInput
	}
	taxType := input.Type
	if taxType == "" {
		taxType = domain.TaxRateTypeOutput
	}
	if taxType != domain.TaxRateTypeOutput && taxType != domain.TaxRateTypeInput && taxType != domain.TaxRateTypeOther {
		return domain.TaxRate{}, domain.ErrInvalidInput
	}
	if code == "" || name == "" {
		return domain.TaxRate{}, domain.ErrInvalidInput
	}

	taxRate := domain.TaxRate{
		ID:       uuid.NewString(),
		TenantID: tenantID,
		Code:     code,
		Name:     name,
		Rate:     rate.String(),
		Type:     taxType,
		Status:   domain.StatusActive,
	}
	if err := s.repo.Create(ctx, taxRate); err != nil {
		return domain.TaxRate{}, err
	}
	return taxRate, nil
}

func (s *TaxRateService) Update(ctx context.Context, id string, input TaxRateInput) (domain.TaxRate, error) {
	tenantID, _, _ := tenantActor(ctx)
	filter := scope.Filter{DataScope: "all"}
	taxRate, err := s.repo.FindByID(ctx, filter, tenantID, id)
	if err != nil {
		return domain.TaxRate{}, err
	}

	code := strings.TrimSpace(input.Code)
	name := strings.TrimSpace(input.Name)
	rate, err := decimal.NewFromString(strings.TrimSpace(input.Rate))
	if err != nil || rate.IsNegative() {
		return domain.TaxRate{}, domain.ErrInvalidInput
	}
	taxType := input.Type
	if taxType == "" {
		taxType = taxRate.Type
	}
	if taxType != domain.TaxRateTypeOutput && taxType != domain.TaxRateTypeInput && taxType != domain.TaxRateTypeOther {
		return domain.TaxRate{}, domain.ErrInvalidInput
	}
	if code == "" || name == "" {
		return domain.TaxRate{}, domain.ErrInvalidInput
	}

	taxRate.Code = code
	taxRate.Name = name
	taxRate.Rate = rate.String()
	taxRate.Type = taxType

	if err := s.repo.Update(ctx, filter, taxRate); err != nil {
		return domain.TaxRate{}, err
	}
	return taxRate, nil
}

func (s *TaxRateService) Disable(ctx context.Context, id string) error {
	tenantID, _, userID := tenantActor(ctx)
	return s.repo.Disable(ctx, scope.Filter{DataScope: "all"}, tenantID, id, userID)
}

func (s *TaxRateService) Enable(ctx context.Context, id string) error {
	tenantID, _, userID := tenantActor(ctx)
	return s.repo.Enable(ctx, scope.Filter{DataScope: "all"}, tenantID, id, userID)
}
