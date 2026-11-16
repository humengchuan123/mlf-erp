package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	masterpostgres "github.com/mlf-erp/mlf-erp/internal/modules/masterdata/infra/postgres"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type PartnerService struct {
	repo  *masterpostgres.Repository[domain.Partner]
	codes CodeGenerator
}

func NewPartnerService(repo *masterpostgres.Repository[domain.Partner], codes CodeGenerator) *PartnerService {
	return &PartnerService{repo: repo, codes: codes}
}

type PartnerInput struct {
	Code     string
	Name     string
	Type     string
	Category string
	Contact  string
	Phone    string
	Email    string
	Address  string
	TaxNo    string
}

func (s *PartnerService) List(ctx context.Context, keyword string, page, pageSize int) ([]domain.Partner, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.repo.List(ctx, scope.FromActor(ctx, tenantID), tenantID, strings.TrimSpace(keyword), (page-1)*pageSize, pageSize)
}

func (s *PartnerService) Get(ctx context.Context, id string) (domain.Partner, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.repo.FindByID(ctx, scope.FromActor(ctx, tenantID), tenantID, id)
}

func (s *PartnerService) Create(ctx context.Context, input PartnerInput) (domain.Partner, error) {
	tenantID, orgID, userID := tenantActor(ctx)
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.Partner{}, domain.ErrInvalidInput
	}
	partnerType := input.Type
	if partnerType == "" {
		partnerType = domain.PartnerTypeBoth
	}
	if partnerType != domain.PartnerTypeCustomer && partnerType != domain.PartnerTypeSupplier && partnerType != domain.PartnerTypeBoth {
		return domain.Partner{}, domain.ErrInvalidInput
	}

	code, err := resolveCode(ctx, s.codes, tenantID, "partner", strings.TrimSpace(input.Code))
	if err != nil {
		return domain.Partner{}, err
	}

	partner := domain.Partner{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		OrgID:     orgID,
		Code:      code,
		Name:      name,
		Type:      partnerType,
		Category:  strings.TrimSpace(input.Category),
		Contact:   strings.TrimSpace(input.Contact),
		Phone:     strings.TrimSpace(input.Phone),
		Email:     strings.TrimSpace(input.Email),
		Address:   strings.TrimSpace(input.Address),
		TaxNo:     strings.TrimSpace(input.TaxNo),
		Status:    domain.StatusActive,
		CreatedBy: userID,
		UpdatedBy: userID,
	}
	if err := s.repo.Create(ctx, partner); err != nil {
		return domain.Partner{}, err
	}
	return partner, nil
}

func (s *PartnerService) Update(ctx context.Context, id string, input PartnerInput) (domain.Partner, error) {
	tenantID, _, userID := tenantActor(ctx)
	filter := scope.FromActor(ctx, tenantID)
	partner, err := s.repo.FindByID(ctx, filter, tenantID, id)
	if err != nil {
		return domain.Partner{}, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.Partner{}, domain.ErrInvalidInput
	}
	partnerType := input.Type
	if partnerType == "" {
		partnerType = partner.Type
	}
	if partnerType != domain.PartnerTypeCustomer && partnerType != domain.PartnerTypeSupplier && partnerType != domain.PartnerTypeBoth {
		return domain.Partner{}, domain.ErrInvalidInput
	}
	if code := strings.TrimSpace(input.Code); code != "" {
		partner.Code = code
	}
	partner.Name = name
	partner.Type = partnerType
	partner.Category = strings.TrimSpace(input.Category)
	partner.Contact = strings.TrimSpace(input.Contact)
	partner.Phone = strings.TrimSpace(input.Phone)
	partner.Email = strings.TrimSpace(input.Email)
	partner.Address = strings.TrimSpace(input.Address)
	partner.TaxNo = strings.TrimSpace(input.TaxNo)
	partner.UpdatedBy = userID

	if err := s.repo.Update(ctx, filter, partner); err != nil {
		return domain.Partner{}, err
	}
	return partner, nil
}

func (s *PartnerService) Disable(ctx context.Context, id string) error {
	tenantID, _, userID := tenantActor(ctx)
	return s.repo.Disable(ctx, scope.FromActor(ctx, tenantID), tenantID, id, userID)
}
