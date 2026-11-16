package app

import (
	"context"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/purchase/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/tenant"
	"github.com/shopspring/decimal"
)

type Service struct {
	repo  domain.Repository
	newID func() string
}

func NewService(repo domain.Repository) *Service {
	return &Service{
		repo:  repo,
		newID: uuid.NewString,
	}
}

type CreateOrderInput struct {
	Code       string
	SupplierID string
	Amount     decimal.Decimal
	Currency   string
}

func (s *Service) Create(ctx context.Context, input CreateOrderInput) (*domain.Order, error) {
	tenantInfo, ok := tenant.From(ctx)
	if !ok || tenantInfo.ID == "" {
		return nil, domain.ErrTenantRequired
	}

	order, err := domain.NewOrder(
		s.newID(),
		tenantInfo.ID,
		tenantInfo.OrgID,
		input.Code,
		input.SupplierID,
		input.Amount,
		input.Currency,
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, order); err != nil {
		return nil, err
	}

	return order, nil
}

func (s *Service) List(ctx context.Context) ([]*domain.Order, error) {
	tenantInfo, ok := tenant.From(ctx)
	if !ok || tenantInfo.ID == "" {
		return nil, domain.ErrTenantRequired
	}

	return s.repo.List(ctx, tenantInfo.ID)
}
