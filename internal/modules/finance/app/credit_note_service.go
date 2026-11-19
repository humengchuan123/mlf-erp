package app

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/finance/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/fx"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type CreditNoteService struct {
	notes domain.CreditNoteRepository
	codes CodeGenerator
	rates fx.RateReader
}

func NewCreditNoteService(notes domain.CreditNoteRepository, codes CodeGenerator, rates fx.RateReader) *CreditNoteService {
	return &CreditNoteService{notes: notes, codes: codes, rates: rates}
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

func (s *CreditNoteService) Allocate(
	ctx context.Context,
	id string,
	allocationDate time.Time,
	allocations []domain.AllocationInput,
) (*domain.CreditNote, error) {
	tenantID, _, userID := tenantActor(ctx)
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrInvalidInput
	}
	if len(allocations) == 0 {
		return nil, domain.ErrInvalidInput
	}
	if allocationDate.IsZero() {
		allocationDate = time.Now()
	}

	seen := make(map[string]struct{}, len(allocations))
	inputs := make([]*domain.AllocationInput, 0, len(allocations))
	for _, item := range allocations {
		invoiceID := strings.TrimSpace(item.InvoiceID)
		amount := item.Amount.Round(4)
		if invoiceID == "" || !amount.IsPositive() {
			return nil, domain.ErrInvalidInput
		}
		if _, err := uuid.Parse(invoiceID); err != nil {
			return nil, domain.ErrInvalidInput
		}
		if _, exists := seen[invoiceID]; exists {
			return nil, domain.ErrInvalidInput
		}
		seen[invoiceID] = struct{}{}
		inputs = append(inputs, &domain.AllocationInput{
			InvoiceID: invoiceID,
			Amount:    amount,
		})
	}

	return s.notes.Allocate(
		ctx,
		scope.FromActor(ctx, tenantID),
		tenantID,
		id,
		userID,
		allocationDate,
		inputs,
	)
}

func (s *CreditNoteService) AllocationCandidates(ctx context.Context, id string) ([]*domain.Invoice, error) {
	tenantID, _, _ := tenantActor(ctx)
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return s.notes.AllocationCandidates(ctx, scope.FromActor(ctx, tenantID), tenantID, id)
}
