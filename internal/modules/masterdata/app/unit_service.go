package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	masterpostgres "github.com/mlf-erp/mlf-erp/internal/modules/masterdata/infra/postgres"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type UnitService struct {
	repo *masterpostgres.Repository[domain.Unit]
}

func NewUnitService(repo *masterpostgres.Repository[domain.Unit]) *UnitService {
	return &UnitService{repo: repo}
}

type UnitInput struct {
	Code      string
	Name      string
	Precision int
}

func (s *UnitService) List(ctx context.Context, keyword string, page, pageSize int) ([]domain.Unit, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.repo.List(ctx, scope.Filter{DataScope: "all"}, tenantID, strings.TrimSpace(keyword), (page-1)*pageSize, pageSize)
}

func (s *UnitService) Get(ctx context.Context, id string) (domain.Unit, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.repo.FindByID(ctx, scope.Filter{DataScope: "all"}, tenantID, id)
}

func (s *UnitService) Create(ctx context.Context, input UnitInput) (domain.Unit, error) {
	tenantID, _, _ := tenantActor(ctx)
	code := strings.TrimSpace(input.Code)
	name := strings.TrimSpace(input.Name)
	if code == "" || name == "" || input.Precision < 0 || input.Precision > 6 {
		return domain.Unit{}, domain.ErrInvalidInput
	}

	unit := domain.Unit{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		Code:      code,
		Name:      name,
		Precision: input.Precision,
		Status:    domain.StatusActive,
	}
	if err := s.repo.Create(ctx, unit); err != nil {
		return domain.Unit{}, err
	}
	return unit, nil
}

func (s *UnitService) Update(ctx context.Context, id string, input UnitInput) (domain.Unit, error) {
	tenantID, _, _ := tenantActor(ctx)
	filter := scope.Filter{DataScope: "all"}
	unit, err := s.repo.FindByID(ctx, filter, tenantID, id)
	if err != nil {
		return domain.Unit{}, err
	}

	code := strings.TrimSpace(input.Code)
	name := strings.TrimSpace(input.Name)
	if code == "" || name == "" || input.Precision < 0 || input.Precision > 6 {
		return domain.Unit{}, domain.ErrInvalidInput
	}
	unit.Code = code
	unit.Name = name
	unit.Precision = input.Precision

	if err := s.repo.Update(ctx, filter, unit); err != nil {
		return domain.Unit{}, err
	}
	return unit, nil
}

func (s *UnitService) Disable(ctx context.Context, id string) error {
	tenantID, _, userID := tenantActor(ctx)
	return s.repo.Disable(ctx, scope.Filter{DataScope: "all"}, tenantID, id, userID)
}
