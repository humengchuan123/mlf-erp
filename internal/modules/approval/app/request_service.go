package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/approval/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type RequestService struct {
	flows    domain.FlowRepository
	requests domain.RequestRepository
}

func NewRequestService(flows domain.FlowRepository, requests domain.RequestRepository) *RequestService {
	return &RequestService{flows: flows, requests: requests}
}

func (s *RequestService) Start(ctx context.Context, input domain.StartInput) (bool, error) {
	tenantID := input.TenantID
	if tenantID == "" {
		tenantID = currentTenantID(ctx)
	}

	flow, err := s.flows.FindMatching(ctx, tenantID, input.OrgID, input.DocumentType, input.BaseAmount)
	if err != nil {
		return false, err
	}
	if flow == nil {
		return false, nil
	}

	steps := make([]domain.FlowStep, 0, len(flow.Steps))
	for _, step := range flow.Steps {
		if step.ApproverType == domain.ApproverUser && step.ApproverID == input.RequestedBy {
			continue
		}
		steps = append(steps, step)
	}
	if len(steps) == 0 {
		return false, domain.ErrNoApprover
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].StepNo < steps[j].StepNo })

	now := time.Now()
	request := &domain.Request{
		ID:           uuid.NewString(),
		TenantID:     tenantID,
		OrgID:        input.OrgID,
		DocumentType: input.DocumentType,
		DocumentID:   input.DocumentID,
		DocumentCode: input.DocumentCode,
		FlowID:       flow.ID,
		Status:       domain.RequestPending,
		CurrentStep:  steps[0].StepNo,
		RequestedBy:  input.RequestedBy,
		RequestedAt:  now,
		Amount:       input.Amount,
		BaseAmount:   input.BaseAmount,
		Currency:     input.Currency,
		CreatedAt:    now,
		UpdatedAt:    now,
		Steps:        steps,
	}
	if err := s.requests.Create(ctx, request); err != nil {
		return false, err
	}
	return true, nil
}

func (s *RequestService) Pending(ctx context.Context, documentType domain.DocumentType, documentID string) (bool, error) {
	request, err := s.requests.GetPendingByDocument(ctx, currentTenantID(ctx), documentType, documentID)
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return request != nil, nil
}

func (s *RequestService) PendingRequest(ctx context.Context, documentType domain.DocumentType, documentID string) (*domain.Request, error) {
	request, err := s.requests.GetPendingByDocument(ctx, currentTenantID(ctx), documentType, documentID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	return request, err
}

func (s *RequestService) Approve(ctx context.Context, documentType domain.DocumentType, documentID string, actor domain.Actor, comment string) (bool, error) {
	tenantID := currentTenantID(ctx)
	request, err := s.requests.GetPendingByDocument(ctx, tenantID, documentType, documentID)
	if errors.Is(err, domain.ErrNotFound) {
		return false, domain.ErrNoPending
	}
	if err != nil {
		return false, err
	}
	if request == nil {
		return false, domain.ErrNoPending
	}
	return s.requests.Approve(ctx, tenantID, request.ID, actor, comment)
}

func (s *RequestService) Reject(ctx context.Context, documentType domain.DocumentType, documentID string, actor domain.Actor, comment string) error {
	tenantID := currentTenantID(ctx)
	request, err := s.requests.GetPendingByDocument(ctx, tenantID, documentType, documentID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ErrNoPending
	}
	if err != nil {
		return err
	}
	if request == nil {
		return domain.ErrNoPending
	}
	return s.requests.Reject(ctx, tenantID, request.ID, actor, comment)
}

func (s *RequestService) Cancel(ctx context.Context, documentType domain.DocumentType, documentID, actorID, reason string) error {
	err := s.requests.CancelByDocument(ctx, currentTenantID(ctx), documentType, documentID, actorID, reason)
	if errors.Is(err, domain.ErrNoPending) {
		return nil
	}
	return err
}

func (s *RequestService) List(ctx context.Context, documentType domain.DocumentType, documentID, status string, page, pageSize int) ([]*domain.Request, int, error) {
	tenantID := currentTenantID(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.requests.List(ctx, scope.FromActor(ctx, tenantID), tenantID, documentType, documentID, strings.TrimSpace(status), page, pageSize)
}

func (s *RequestService) ListPending(ctx context.Context, actor domain.Actor) ([]*domain.Request, error) {
	tenantID := currentTenantID(ctx)
	return s.requests.ListPending(ctx, scope.FromActor(ctx, tenantID), tenantID, actor)
}

func (s *RequestService) Get(ctx context.Context, id string) (*domain.Request, error) {
	tenantID := currentTenantID(ctx)
	return s.requests.Get(ctx, scope.FromActor(ctx, tenantID), tenantID, id)
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}
