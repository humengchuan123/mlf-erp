package domain

import (
	"context"

	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type InvoiceRepository interface {
	List(ctx context.Context, filter scope.Filter, tenantID, invoiceType, status, vatInvoiceStatus, partnerID, keyword string, offset, limit int) ([]*Invoice, int, error)
	Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*Invoice, error)
	ListAllocations(ctx context.Context, filter scope.Filter, tenantID, invoiceID string) ([]*Allocation, error)
	Issue(ctx context.Context, filter scope.Filter, tenantID, id string, input IssueInput) (*Invoice, error)
	Void(ctx context.Context, filter scope.Filter, tenantID, id, userID string) (*Invoice, error)
}

type PaymentRepository interface {
	Create(ctx context.Context, payment *Payment) error
	Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*Payment, error)
	List(ctx context.Context, filter scope.Filter, tenantID, paymentType, status, partnerID, keyword string, offset, limit int) ([]*Payment, int, error)
	Allocate(ctx context.Context, filter scope.Filter, tenantID, paymentID, userID string, allocations []*AllocationInput) error
}
