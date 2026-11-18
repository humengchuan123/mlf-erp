package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/finance/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type CreditNoteService struct {
	notes domain.CreditNoteRepository
}

func NewCreditNoteService(notes domain.CreditNoteRepository) *CreditNoteService {
	return &CreditNoteService{notes: notes}
}

func (s *CreditNoteService) List(
	ctx context.Context,
	creditType, status, partnerID, keyword string,
	page, pageSize int,
) ([]*domain.CreditNote, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	creditType = normalizeString(creditType)
	if creditType != "" &&
		creditType != domain.CreditNoteTypeSalesReturn &&
		creditType != domain.CreditNoteTypePurchaseReturn {
		return nil, 0, domain.ErrInvalidInput
	}
	status = normalizeString(status)
	if status != "" && status != domain.CreditNoteStatusOpen && status != domain.CreditNoteStatusApplied {
		return nil, 0, domain.ErrInvalidInput
	}
	partnerID = normalizeString(partnerID)
	if partnerID != "" {
		if _, err := uuid.Parse(partnerID); err != nil {
			return nil, 0, domain.ErrInvalidInput
		}
	}
	return s.notes.List(
		ctx,
		scope.FromActor(ctx, tenantID),
		tenantID,
		creditType,
		status,
		partnerID,
		strings.TrimSpace(keyword),
		(page-1)*pageSize,
		pageSize,
	)
}

func (s *CreditNoteService) Get(ctx context.Context, id string) (*domain.CreditNote, error) {
	tenantID, _, _ := tenantActor(ctx)
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return s.notes.Get(ctx, scope.FromActor(ctx, tenantID), tenantID, id)
}
