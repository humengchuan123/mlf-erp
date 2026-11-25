package app

import (
	"context"
	"strings"

	"github.com/mlf-erp/mlf-erp/internal/modules/finance/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type InvoiceService struct {
	invoices domain.InvoiceRepository
}

func NewInvoiceService(invoices domain.InvoiceRepository) *InvoiceService {
	return &InvoiceService{invoices: invoices}
}

type InvoiceDetail struct {
	Invoice                    *domain.Invoice
	Allocations                []*domain.Allocation
	CreditNoteAllocations      []*domain.CreditNoteAllocation
	PaymentAllocationReversals []*domain.PaymentAllocationReversal
}

func (s *InvoiceService) List(
	ctx context.Context,
	invoiceType, status, vatInvoiceStatus, partnerID, keyword string,
	page, pageSize int,
) ([]*domain.Invoice, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.invoices.List(
		ctx,
		scope.FromActor(ctx, tenantID),
		tenantID,
		normalizeString(invoiceType),
		normalizeString(status),
		normalizeString(vatInvoiceStatus),
		normalizeString(partnerID),
		strings.TrimSpace(keyword),
		(page-1)*pageSize,
		pageSize,
	)
}

func (s *InvoiceService) Get(ctx context.Context, id string) (*InvoiceDetail, error) {
	tenantID, _, _ := tenantActor(ctx)
	filter := scope.FromActor(ctx, tenantID)
	invoice, err := s.invoices.Get(ctx, filter, tenantID, id)
	if err != nil {
		return nil, err
	}
	allocations, err := s.invoices.ListAllocations(ctx, filter, tenantID, id)
	if err != nil {
		return nil, err
	}
	creditNoteAllocations, err := s.invoices.ListCreditNoteAllocations(ctx, filter, tenantID, id)
	if err != nil {
		return nil, err
	}
	paymentAllocationReversals, err := s.invoices.ListPaymentAllocationReversals(ctx, filter, tenantID, id)
	if err != nil {
		return nil, err
	}
	return &InvoiceDetail{
		Invoice:                    invoice,
		Allocations:                allocations,
		CreditNoteAllocations:      creditNoteAllocations,
		PaymentAllocationReversals: paymentAllocationReversals,
	}, nil
}

func (s *InvoiceService) Issue(ctx context.Context, id string, input domain.IssueInput) (*domain.Invoice, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.invoices.Issue(ctx, scope.FromActor(ctx, tenantID), tenantID, id, input)
}

func (s *InvoiceService) Void(ctx context.Context, id string) (*domain.Invoice, error) {
	tenantID, _, userID := tenantActor(ctx)
	return s.invoices.Void(ctx, scope.FromActor(ctx, tenantID), tenantID, id, userID)
}
