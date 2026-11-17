package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
)

type AccountService struct {
	accounts domain.AccountRepository
}

func NewAccountService(accounts domain.AccountRepository) *AccountService {
	return &AccountService{accounts: accounts}
}

type AccountInput struct {
	Code      string
	Name      string
	Type      string
	Direction string
	ParentID  string
	Status    string
}

func (s *AccountService) List(ctx context.Context) ([]*domain.Account, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.accounts.List(ctx, tenantID)
}

func (s *AccountService) Create(ctx context.Context, input AccountInput) (*domain.Account, error) {
	tenantID, _, _ := tenantActor(ctx)
	account, err := s.build(ctx, tenantID, "", input)
	if err != nil {
		return nil, err
	}
	if err := s.accounts.Create(ctx, account); err != nil {
		return nil, err
	}
	return account, nil
}

func (s *AccountService) Update(ctx context.Context, id string, input AccountInput) (*domain.Account, error) {
	tenantID, _, _ := tenantActor(ctx)
	existing, err := s.accounts.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	account, err := s.build(ctx, tenantID, id, input)
	if err != nil {
		return nil, err
	}
	account.CreatedAt = existing.CreatedAt
	if err := s.accounts.Update(ctx, account); err != nil {
		return nil, err
	}
	return account, nil
}

func (s *AccountService) build(ctx context.Context, tenantID, id string, input AccountInput) (*domain.Account, error) {
	code := trim(input.Code)
	name := trim(input.Name)
	accountType := strings.ToLower(trim(input.Type))
	direction := strings.ToLower(trim(input.Direction))
	if code == "" || name == "" {
		return nil, domain.ErrInvalidInput
	}
	switch accountType {
	case domain.AccountTypeAsset, domain.AccountTypeLiability, domain.AccountTypeEquity,
		domain.AccountTypeRevenue, domain.AccountTypeExpense:
	default:
		return nil, domain.ErrInvalidInput
	}
	if direction != domain.DirectionDebit && direction != domain.DirectionCredit {
		return nil, domain.ErrInvalidInput
	}

	level := 1
	parentID := trim(input.ParentID)
	if parentID != "" {
		parent, err := s.accounts.Get(ctx, tenantID, parentID)
		if err != nil {
			return nil, domain.ErrInvalidInput
		}
		level = parent.Level + 1
	}
	status := strings.ToLower(trim(input.Status))
	if status == "" {
		status = domain.StatusActive
	}
	if status != domain.StatusActive && status != domain.StatusDisabled {
		return nil, domain.ErrInvalidInput
	}

	if id == "" {
		id = uuid.NewString()
	}
	return &domain.Account{
		ID:        id,
		TenantID:  tenantID,
		Code:      code,
		Name:      name,
		Type:      accountType,
		Direction: direction,
		ParentID:  parentID,
		Level:     level,
		IsLeaf:    true,
		Status:    status,
	}, nil
}
