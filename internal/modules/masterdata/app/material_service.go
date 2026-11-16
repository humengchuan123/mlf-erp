package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type MaterialService struct {
	repo     domain.CrudRepository[domain.Material]
	units    domain.CrudRepository[domain.Unit]
	taxRates domain.CrudRepository[domain.TaxRate]
	codes    CodeGenerator
}

func NewMaterialService(
	repo domain.CrudRepository[domain.Material],
	units domain.CrudRepository[domain.Unit],
	taxRates domain.CrudRepository[domain.TaxRate],
	codes CodeGenerator,
) *MaterialService {
	return &MaterialService{repo: repo, units: units, taxRates: taxRates, codes: codes}
}

type MaterialInput struct {
	Code      string
	Name      string
	Spec      string
	Category  string
	UnitID    string
	TaxRateID string
	Barcode   string
}

func (s *MaterialService) List(ctx context.Context, keyword string, page, pageSize int) ([]domain.Material, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.repo.List(ctx, scope.FromActor(ctx, tenantID), tenantID, strings.TrimSpace(keyword), (page-1)*pageSize, pageSize)
}

func (s *MaterialService) Get(ctx context.Context, id string) (domain.Material, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.repo.FindByID(ctx, scope.FromActor(ctx, tenantID), tenantID, id)
}

func (s *MaterialService) Create(ctx context.Context, input MaterialInput) (domain.Material, error) {
	tenantID, orgID, userID := tenantActor(ctx)
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.Material{}, domain.ErrInvalidInput
	}
	if err := s.validateReferences(ctx, tenantID, input.UnitID, input.TaxRateID); err != nil {
		return domain.Material{}, err
	}

	code, err := resolveCode(ctx, s.codes, tenantID, "material", strings.TrimSpace(input.Code))
	if err != nil {
		return domain.Material{}, err
	}

	material := domain.Material{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		OrgID:     orgID,
		Code:      code,
		Name:      name,
		Spec:      strings.TrimSpace(input.Spec),
		Category:  strings.TrimSpace(input.Category),
		UnitID:    strings.TrimSpace(input.UnitID),
		TaxRateID: strings.TrimSpace(input.TaxRateID),
		Barcode:   strings.TrimSpace(input.Barcode),
		Status:    domain.StatusActive,
		CreatedBy: userID,
		UpdatedBy: userID,
	}
	if err := s.repo.Create(ctx, material); err != nil {
		return domain.Material{}, err
	}
	return material, nil
}

func (s *MaterialService) Update(ctx context.Context, id string, input MaterialInput) (domain.Material, error) {
	tenantID, _, userID := tenantActor(ctx)
	filter := scope.FromActor(ctx, tenantID)
	material, err := s.repo.FindByID(ctx, filter, tenantID, id)
	if err != nil {
		return domain.Material{}, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.Material{}, domain.ErrInvalidInput
	}
	nextUnitID := strings.TrimSpace(input.UnitID)
	nextTaxRateID := strings.TrimSpace(input.TaxRateID)
	if nextUnitID != material.UnitID || nextTaxRateID != material.TaxRateID {
		if err := s.validateReferences(ctx, tenantID, nextUnitID, nextTaxRateID); err != nil {
			return domain.Material{}, err
		}
	}
	if code := strings.TrimSpace(input.Code); code != "" {
		material.Code = code
	}
	material.Name = name
	material.Spec = strings.TrimSpace(input.Spec)
	material.Category = strings.TrimSpace(input.Category)
	material.UnitID = nextUnitID
	material.TaxRateID = nextTaxRateID
	material.Barcode = strings.TrimSpace(input.Barcode)
	material.UpdatedBy = userID

	if err := s.repo.Update(ctx, filter, material); err != nil {
		return domain.Material{}, err
	}
	return material, nil
}

func (s *MaterialService) Disable(ctx context.Context, id string) error {
	tenantID, _, userID := tenantActor(ctx)
	return s.repo.Disable(ctx, scope.FromActor(ctx, tenantID), tenantID, id, userID)
}

func (s *MaterialService) Enable(ctx context.Context, id string) error {
	tenantID, _, userID := tenantActor(ctx)
	return s.repo.Enable(ctx, scope.FromActor(ctx, tenantID), tenantID, id, userID)
}

func (s *MaterialService) validateReferences(ctx context.Context, tenantID, unitID, taxRateID string) error {
	filter := scope.Filter{TenantID: tenantID, DataScope: "all"}
	if unitID = strings.TrimSpace(unitID); unitID != "" {
		unit, err := s.units.FindByID(ctx, filter, tenantID, unitID)
		if err != nil || unit.Status != domain.StatusActive {
			return domain.ErrInvalidInput
		}
	}
	if taxRateID = strings.TrimSpace(taxRateID); taxRateID != "" {
		taxRate, err := s.taxRates.FindByID(ctx, filter, tenantID, taxRateID)
		if err != nil || taxRate.Status != domain.StatusActive {
			return domain.ErrInvalidInput
		}
	}
	return nil
}
