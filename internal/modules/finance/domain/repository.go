package domain

import (
	"context"
	"time"

	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type InvoiceRepository interface {
	List(ctx context.Context, filter scope.Filter, tenantID, invoiceType, status, vatInvoiceStatus, partnerID, keyword string, offset, limit int) ([]*Invoice, int, error)
	Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*Invoice, error)
	ListAllocations(ctx context.Context, filter scope.Filter, tenantID, invoiceID string) ([]*Allocation, error)
	Issue(ctx context.Context, filter scope.Filter, tenantID, id string, input IssueInput) (*Invoice, error)
	Void(ctx context.Context, filter scope.Filter, tenantID, id, userID string) (*Invoice, error)
}

type CreditNoteRepository interface {
	List(ctx context.Context, filter scope.Filter, tenantID, creditType, status, partnerID, keyword string, offset, limit int) ([]*CreditNote, int, error)
	Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*CreditNote, error)
	Allocate(ctx context.Context, filter scope.Filter, tenantID, creditNoteID, userID string,
		allocationDate time.Time, allocations []*AllocationInput) (*CreditNote, error)
	AllocationCandidates(ctx context.Context, filter scope.Filter, tenantID, creditNoteID string) ([]*Invoice, error)
}

type PaymentRepository interface {
	Create(ctx context.Context, payment *Payment) error
	Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*Payment, error)
	List(ctx context.Context, filter scope.Filter, tenantID, paymentType, status, partnerID, keyword string, offset, limit int) ([]*Payment, int, error)
	Allocate(ctx context.Context, filter scope.Filter, tenantID, paymentID, userID string, allocations []*AllocationInput) error
}

type ExchangeRateRepository interface {
	List(ctx context.Context, tenantID, currency string, startDate, endDate time.Time, offset, limit int) ([]*ExchangeRate, int, error)
	Get(ctx context.Context, tenantID, id string) (*ExchangeRate, error)
	Upsert(ctx context.Context, tenantID, userID string, input ExchangeRateInput) (*ExchangeRate, error)
	Update(ctx context.Context, tenantID, id, userID string, input ExchangeRateInput) (*ExchangeRate, error)
	Latest(ctx context.Context, tenantID, currency string, date time.Time) (decimal.Decimal, error)
}
