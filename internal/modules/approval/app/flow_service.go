package app

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/approval/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/tenant"
	"github.com/shopspring/decimal"
)

type FlowService struct {
	flows     domain.FlowRepository
	approvers domain.ApproverValidator
}

func NewFlowService(flows domain.FlowRepository, approvers domain.ApproverValidator) *FlowService {
	return &FlowService{flows: flows, approvers: approvers}
}

type FlowStepInput struct {
	StepNo       int
	Name         string
	ApproverType domain.ApproverType
	ApproverID   string
}

type FlowInput struct {
	Name            string
	DocumentType    domain.DocumentType
	OrgID           string
	ConditionType   domain.ConditionType
	ThresholdAmount decimal.Decimal
	Priority        int
	Active          bool
	Steps           []FlowStepInput
}

func (s *FlowService) List(ctx context.Context, documentType domain.DocumentType, active *bool) ([]*domain.Flow, error) {
	return s.flows.List(ctx, currentTenantID(ctx), documentType, active)
}

func (s *FlowService) Get(ctx context.Context, id string) (*domain.Flow, error) {
	return s.flows.Get(ctx, currentTenantID(ctx), id)
}

func (s *FlowService) Create(ctx context.Context, input FlowInput) (*domain.Flow, error) {
	tenantID := currentTenantID(ctx)
	userID := currentUserID(ctx)
	flow, err := s.buildFlow(ctx, tenantID, input)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	flow.ID = uuid.NewString()
	attachFlowSteps(flow, tenantID, now)
	flow.CreatedBy = userID
	flow.UpdatedBy = userID
	flow.CreatedAt = now
	flow.UpdatedAt = now

	if err := s.flows.Create(ctx, flow); err != nil {
		return nil, err
	}
	return flow, nil
}

func (s *FlowService) Update(ctx context.Context, id string, input FlowInput) (*domain.Flow, error) {
	tenantID := currentTenantID(ctx)
	userID := currentUserID(ctx)
	existing, err := s.flows.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	inUse, err := s.flows.HasRequests(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if inUse {
		return nil, domain.ErrFlowInUse
	}

	flow, err := s.buildFlow(ctx, tenantID, input)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	flow.ID = id
	attachFlowSteps(flow, tenantID, now)
	flow.CreatedBy = existing.CreatedBy
	flow.CreatedAt = existing.CreatedAt
	flow.UpdatedBy = userID
	flow.UpdatedAt = now

	if err := s.flows.Update(ctx, flow); err != nil {
		return nil, err
	}
	return flow, nil
}

// SetActive enables or disables a flow. Unlike Update/Delete it is allowed even
// when the flow has approval request history, so operators can retire a flow
// and create a replacement.
func (s *FlowService) SetActive(ctx context.Context, id string, active bool) (*domain.Flow, error) {
	tenantID := currentTenantID(ctx)
	userID := currentUserID(ctx)
	if err := s.flows.SetActive(ctx, tenantID, id, active, userID); err != nil {
		return nil, err
	}
	return s.flows.Get(ctx, tenantID, id)
}

func (s *FlowService) Delete(ctx context.Context, id string) error {
	tenantID := currentTenantID(ctx)
	if _, err := s.flows.Get(ctx, tenantID, id); err != nil {
		return err
	}
	inUse, err := s.flows.HasRequests(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if inUse {
		return domain.ErrFlowInUse
	}
	return s.flows.Delete(ctx, tenantID, id)
}

func (s *FlowService) buildFlow(ctx context.Context, tenantID string, input FlowInput) (*domain.Flow, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, domain.ErrInvalidFlow
	}
	switch input.DocumentType {
	case domain.DocumentSalesOrder, domain.DocumentPurchaseOrder:
	default:
		return nil, domain.ErrInvalidFlow
	}
	switch input.ConditionType {
	case domain.ConditionAlways, domain.ConditionAmountGTE:
	default:
		return nil, domain.ErrInvalidFlow
	}
	if input.ThresholdAmount.IsNegative() {
		return nil, domain.ErrInvalidFlow
	}
	if len(input.Steps) == 0 {
		return nil, domain.ErrInvalidFlow
	}

	steps := make([]domain.FlowStep, len(input.Steps))
	seen := make(map[int]struct{}, len(input.Steps))
	for i, stepInput := range input.Steps {
		if stepInput.StepNo < 1 || stepInput.StepNo > len(input.Steps) {
			return nil, domain.ErrInvalidFlow
		}
		if _, ok := seen[stepInput.StepNo]; ok {
			return nil, domain.ErrInvalidFlow
		}
		seen[stepInput.StepNo] = struct{}{}

		approverID := strings.TrimSpace(stepInput.ApproverID)
		if approverID == "" {
			return nil, domain.ErrInvalidFlow
		}
		var exists bool
		var err error
		switch stepInput.ApproverType {
		case domain.ApproverRole:
			exists, err = s.approvers.RoleExists(ctx, tenantID, approverID)
		case domain.ApproverUser:
			exists, err = s.approvers.UserExists(ctx, tenantID, approverID)
		default:
			return nil, domain.ErrInvalidFlow
		}
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, domain.ErrInvalidFlow
		}

		steps[i] = domain.FlowStep{
			StepNo:       stepInput.StepNo,
			Name:         strings.TrimSpace(stepInput.Name),
			ApproverType: stepInput.ApproverType,
			ApproverID:   approverID,
		}
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].StepNo < steps[j].StepNo })

	return &domain.Flow{
		TenantID:        tenantID,
		OrgID:           strings.TrimSpace(input.OrgID),
		Name:            name,
		DocumentType:    input.DocumentType,
		ConditionType:   input.ConditionType,
		ThresholdAmount: input.ThresholdAmount,
		Priority:        input.Priority,
		Active:          input.Active,
		Steps:           steps,
	}, nil
}

func attachFlowSteps(flow *domain.Flow, tenantID string, now time.Time) {
	for i := range flow.Steps {
		flow.Steps[i].ID = uuid.NewString()
		flow.Steps[i].TenantID = tenantID
		flow.Steps[i].FlowID = flow.ID
		flow.Steps[i].CreatedAt = now
	}
}

func currentTenantID(ctx context.Context) string {
	info, _ := tenant.From(ctx)
	return info.ID
}

func currentUserID(ctx context.Context) string {
	actor, _ := auth.ActorFrom(ctx)
	return actor.UserID
}
