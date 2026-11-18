package app

import (
	"context"
	"strings"

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
	status, partnerID, keyword string,
	page, pageSize int,
) ([]*domain.CreditNote, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.notes.List(
		ctx,
		scope.FromActor(ctx, tenantID),
		tenantID,
		normalizeString(status),
		normalizeString(partnerID),
		strings.TrimSpace(keyword),
		(page-1)*pageSize,
		pageSize,
	)
}

func (s *CreditNoteService) Get(ctx context.Context, id string) (*domain.CreditNote, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.notes.Get(ctx, scope.FromActor(ctx, tenantID), tenantID, id)
}
