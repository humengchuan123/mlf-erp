package app

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/tenant"
)

type AuditService struct {
	repo domain.AuditRepository
}

func NewAuditService(repo domain.AuditRepository) *AuditService {
	return &AuditService{repo: repo}
}

type AuditInput struct {
	Action       string
	ResourceType string
	ResourceID   string
	RequestID    string
	IP           string
	UserAgent    string
	Before       any
	After        any
}

func (s *AuditService) Record(ctx context.Context, input AuditInput) error {
	tenantInfo, ok := tenant.From(ctx)
	if !ok || tenantInfo.ID == "" {
		return nil
	}

	actor, _ := auth.ActorFrom(ctx)
	return s.repo.Create(ctx, &domain.AuditLog{
		ID:           uuid.NewString(),
		TenantID:     tenantInfo.ID,
		OrgID:        tenantInfo.OrgID,
		UserID:       actor.UserID,
		Action:       input.Action,
		ResourceType: input.ResourceType,
		ResourceID:   input.ResourceID,
		RequestID:    input.RequestID,
		IP:           input.IP,
		UserAgent:    input.UserAgent,
		BeforeData:   marshalAuditData(input.Before),
		AfterData:    marshalAuditData(input.After),
	})
}

func (s *AuditService) List(ctx context.Context, tenantID string, page, pageSize int) ([]*domain.AuditLog, int, error) {
	page, pageSize = normalizePage(page, pageSize)
	return s.repo.List(ctx, tenantID, (page-1)*pageSize, pageSize)
}

func marshalAuditData(data any) []byte {
	if data == nil {
		return nil
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	return encoded
}
