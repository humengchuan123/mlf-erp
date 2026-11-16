package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	masterpostgres "github.com/mlf-erp/mlf-erp/internal/modules/masterdata/infra/postgres"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type WarehouseService struct {
	repo  *masterpostgres.Repository[domain.Warehouse]
	codes CodeGenerator
}

func NewWarehouseService(repo *masterpostgres.Repository[domain.Warehouse], codes CodeGenerator) *WarehouseService {
	return &WarehouseService{repo: repo, codes: codes}
}

type WarehouseInput struct {
	Code    string
	Name    string
	Type    string
	Address string
}

func (s *WarehouseService) List(ctx context.Context, keyword string, page, pageSize int) ([]domain.Warehouse, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.repo.List(ctx, scope.FromActor(ctx, tenantID), tenantID, strings.TrimSpace(keyword), (page-1)*pageSize, pageSize)
}

func (s *WarehouseService) Get(ctx context.Context, id string) (domain.Warehouse, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.repo.FindByID(ctx, scope.FromActor(ctx, tenantID), tenantID, id)
}

func (s *WarehouseService) Create(ctx context.Context, input WarehouseInput) (domain.Warehouse, error) {
	tenantID, orgID, userID := tenantActor(ctx)
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.Warehouse{}, domain.ErrInvalidInput
	}
	warehouseType := input.Type
	if warehouseType == "" {
		warehouseType = domain.WarehouseTypeNormal
	}
	if warehouseType != domain.WarehouseTypeNormal && warehouseType != domain.WarehouseTypeConsignment && warehouseType != domain.WarehouseTypeOther {
		return domain.Warehouse{}, domain.ErrInvalidInput
	}

	code, err := resolveCode(ctx, s.codes, tenantID, "warehouse", strings.TrimSpace(input.Code))
	if err != nil {
		return domain.Warehouse{}, err
	}

	warehouse := domain.Warehouse{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		OrgID:     orgID,
		Code:      code,
		Name:      name,
		Type:      warehouseType,
		Address:   strings.TrimSpace(input.Address),
		Status:    domain.StatusActive,
		CreatedBy: userID,
		UpdatedBy: userID,
	}
	if err := s.repo.Create(ctx, warehouse); err != nil {
		return domain.Warehouse{}, err
	}
	return warehouse, nil
}

func (s *WarehouseService) Update(ctx context.Context, id string, input WarehouseInput) (domain.Warehouse, error) {
	tenantID, _, userID := tenantActor(ctx)
	filter := scope.FromActor(ctx, tenantID)
	warehouse, err := s.repo.FindByID(ctx, filter, tenantID, id)
	if err != nil {
		return domain.Warehouse{}, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.Warehouse{}, domain.ErrInvalidInput
	}
	warehouseType := input.Type
	if warehouseType == "" {
		warehouseType = warehouse.Type
	}
	if warehouseType != domain.WarehouseTypeNormal && warehouseType != domain.WarehouseTypeConsignment && warehouseType != domain.WarehouseTypeOther {
		return domain.Warehouse{}, domain.ErrInvalidInput
	}
	if code := strings.TrimSpace(input.Code); code != "" {
		warehouse.Code = code
	}
	warehouse.Name = name
	warehouse.Type = warehouseType
	warehouse.Address = strings.TrimSpace(input.Address)
	warehouse.UpdatedBy = userID

	if err := s.repo.Update(ctx, filter, warehouse); err != nil {
		return domain.Warehouse{}, err
	}
	return warehouse, nil
}

func (s *WarehouseService) Disable(ctx context.Context, id string) error {
	tenantID, _, userID := tenantActor(ctx)
	return s.repo.Disable(ctx, scope.FromActor(ctx, tenantID), tenantID, id, userID)
}
