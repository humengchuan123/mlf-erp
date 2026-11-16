package app

import (
	"context"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
)

type OrgService struct {
	orgs domain.OrgRepository
}

func NewOrgService(orgs domain.OrgRepository) *OrgService {
	return &OrgService{orgs: orgs}
}

type CreateOrgInput struct {
	TenantID string
	ParentID string
	Type     string
	Code     string
	Name     string
}

type UpdateOrgInput struct {
	ParentID string
	Type     string
	Code     string
	Name     string
	Status   string
}

func (s *OrgService) ListTree(ctx context.Context, tenantID string) ([]*domain.Org, error) {
	orgs, err := s.orgs.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]*domain.Org, len(orgs))
	for _, org := range orgs {
		org.Children = make([]*domain.Org, 0)
		byID[org.ID] = org
	}

	roots := make([]*domain.Org, 0)
	for _, org := range orgs {
		if org.ParentID == "" {
			roots = append(roots, org)
			continue
		}
		parent, ok := byID[org.ParentID]
		if !ok {
			roots = append(roots, org)
			continue
		}
		parent.Children = append(parent.Children, org)
	}

	sortOrgs(roots)
	return roots, nil
}

func (s *OrgService) Create(ctx context.Context, input CreateOrgInput) (*domain.Org, error) {
	code := strings.TrimSpace(input.Code)
	name := strings.TrimSpace(input.Name)
	if code == "" || name == "" {
		return nil, domain.ErrConflict
	}

	id := uuid.NewString()
	path := "/" + id
	if input.ParentID != "" {
		parent, err := s.orgs.FindByID(ctx, input.TenantID, input.ParentID)
		if err != nil {
			return nil, err
		}
		path = strings.TrimRight(parent.Path, "/") + "/" + id
	}

	orgType := input.Type
	if orgType == "" {
		orgType = domain.OrgTypeDept
	}

	org := &domain.Org{
		ID:       id,
		TenantID: input.TenantID,
		ParentID: input.ParentID,
		Type:     orgType,
		Code:     code,
		Name:     name,
		Path:     path,
		Status:   domain.OrgStatusActive,
	}
	if err := s.orgs.Create(ctx, org); err != nil {
		return nil, err
	}
	return org, nil
}

func (s *OrgService) Update(ctx context.Context, tenantID, id string, input UpdateOrgInput) (*domain.Org, error) {
	org, err := s.orgs.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	code := strings.TrimSpace(input.Code)
	name := strings.TrimSpace(input.Name)
	if code == "" || name == "" {
		return nil, domain.ErrConflict
	}

	if input.ParentID != org.ParentID {
		if input.ParentID == org.ID {
			return nil, domain.ErrInvalidInput
		}

		orgs, listErr := s.orgs.List(ctx, tenantID)
		if listErr != nil {
			return nil, listErr
		}
		for _, item := range orgs {
			if item.ParentID == org.ID {
				return nil, domain.ErrOrgHasChildren
			}
		}

		if input.ParentID == "" {
			org.Path = "/" + org.ID
		} else {
			parent, parentErr := s.orgs.FindByID(ctx, tenantID, input.ParentID)
			if parentErr != nil {
				return nil, parentErr
			}
			if parent.Path == org.Path || strings.HasPrefix(parent.Path, org.Path+"/") {
				return nil, domain.ErrInvalidInput
			}
			org.Path = strings.TrimRight(parent.Path, "/") + "/" + org.ID
		}
		org.ParentID = input.ParentID
	}

	org.Code = code
	org.Name = name
	if input.Type != "" {
		org.Type = input.Type
	}
	if input.Status != "" {
		org.Status = input.Status
	}

	if err := s.orgs.Update(ctx, org); err != nil {
		return nil, err
	}
	return org, nil
}

func (s *OrgService) Delete(ctx context.Context, tenantID, id string) error {
	return s.orgs.Delete(ctx, tenantID, id)
}

func sortOrgs(orgs []*domain.Org) {
	sort.Slice(orgs, func(i, j int) bool {
		return orgs[i].Code < orgs[j].Code
	})
	for _, org := range orgs {
		sortOrgs(org.Children)
	}
}
