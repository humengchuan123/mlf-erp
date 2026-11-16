package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
)

type CodeRuleService struct {
	repo domain.CodeRuleRepository
}

func NewCodeRuleService(repo domain.CodeRuleRepository) *CodeRuleService {
	return &CodeRuleService{repo: repo}
}

type CodeRuleInput struct {
	Entity         string
	Prefix         string
	DatePattern    string
	Separator      string
	SequenceLength int
	ResetPolicy    string
	Status         string
}

func (s *CodeRuleService) List(ctx context.Context, tenantID string) ([]*domain.CodeRule, error) {
	return s.repo.List(ctx, tenantID)
}

func (s *CodeRuleService) Create(ctx context.Context, tenantID string, input CodeRuleInput) (*domain.CodeRule, error) {
	rule, err := normalizeCodeRule(input, tenantID, uuid.NewString())
	if err != nil {
		return nil, err
	}
	rule.TenantID = tenantID
	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *CodeRuleService) Update(ctx context.Context, tenantID, id string, input CodeRuleInput) (*domain.CodeRule, error) {
	rules, err := s.repo.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	var existing *domain.CodeRule
	for _, rule := range rules {
		if rule.ID == id {
			existing = rule
			break
		}
	}
	if existing == nil {
		return nil, domain.ErrNotFound
	}

	updated, err := normalizeCodeRule(input, tenantID, id)
	if err != nil {
		return nil, err
	}
	updated.CurrentValue = existing.CurrentValue
	updated.LastResetDate = existing.LastResetDate
	updated.CreatedAt = existing.CreatedAt

	if err := s.repo.Update(ctx, updated); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *CodeRuleService) Next(ctx context.Context, tenantID, entity string) (string, error) {
	return s.repo.Next(ctx, tenantID, entity)
}

func normalizeCodeRule(input CodeRuleInput, tenantID, id string) (*domain.CodeRule, error) {
	entity := strings.TrimSpace(input.Entity)
	prefix := strings.TrimSpace(input.Prefix)
	if entity == "" || prefix == "" {
		return nil, domain.ErrInvalidInput
	}
	if input.SequenceLength < 1 || input.SequenceLength > 10 {
		return nil, domain.ErrInvalidInput
	}

	datePattern := input.DatePattern
	if datePattern == "" {
		datePattern = domain.DatePatternNone
	}
	switch datePattern {
	case domain.DatePatternNone, domain.DatePatternYear, domain.DatePatternMonth, domain.DatePatternDay:
	default:
		return nil, domain.ErrInvalidInput
	}

	resetPolicy := input.ResetPolicy
	if resetPolicy == "" {
		resetPolicy = domain.ResetPolicyNever
	}
	switch resetPolicy {
	case domain.ResetPolicyNever:
	case domain.ResetPolicyYear:
		if datePattern == domain.DatePatternNone {
			return nil, domain.ErrInvalidInput
		}
	case domain.ResetPolicyMonth:
		if datePattern != domain.DatePatternMonth && datePattern != domain.DatePatternDay {
			return nil, domain.ErrInvalidInput
		}
	case domain.ResetPolicyDay:
		if datePattern != domain.DatePatternDay {
			return nil, domain.ErrInvalidInput
		}
	default:
		return nil, domain.ErrInvalidInput
	}

	status := input.Status
	if status == "" {
		status = domain.CodeRuleStatusActive
	}
	if status != domain.CodeRuleStatusActive && status != domain.CodeRuleStatusDisabled {
		return nil, domain.ErrInvalidInput
	}

	separator := input.Separator
	if separator == "" {
		separator = "-"
	}

	return &domain.CodeRule{
		ID:             id,
		TenantID:       tenantID,
		Entity:         entity,
		Prefix:         prefix,
		DatePattern:    datePattern,
		Separator:      separator,
		SequenceLength: input.SequenceLength,
		ResetPolicy:    resetPolicy,
		Status:         status,
	}, nil
}
