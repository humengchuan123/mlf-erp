package app

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/finance/domain"
	masterdomain "github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/fx"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type PaymentService struct {
	payments domain.PaymentRepository
	codes    CodeGenerator
	partners PartnerReader
	rates    fx.RateReader
}

func NewPaymentService(payments domain.PaymentRepository, codes CodeGenerator, partners PartnerReader, rates fx.RateReader) *PaymentService {
	return &PaymentService{payments: payments, codes: codes, partners: partners, rates: rates}
}

type PaymentInput struct {
	PaymentType  string
	PartnerID    string
	PaymentDate  time.Time
	Currency     string
	ExchangeRate decimal.Decimal
	Amount       decimal.Decimal
	Method       string
	Reference    string
	Remark       string
}

type AllocateInput struct {
	Allocations []domain.AllocationInput
}

type PaymentDetail struct {
	Payment     *domain.Payment
	Allocations []*domain.PaymentAllocation
	Reversals   []*domain.PaymentAllocationReversal
}

func (s *PaymentService) List(
	ctx context.Context,
	paymentType, status, partnerID, keyword string,
	page, pageSize int,
) ([]*domain.Payment, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.payments.List(
		ctx,
		scope.FromActor(ctx, tenantID),
		tenantID,
		normalizeString(paymentType),
		normalizeString(status),
		normalizeString(partnerID),
		strings.TrimSpace(keyword),
		(page-1)*pageSize,
		pageSize,
	)
}

func (s *PaymentService) Get(ctx context.Context, id string) (*domain.Payment, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.payments.Get(ctx, scope.FromActor(ctx, tenantID), tenantID, id)
}

func (s *PaymentService) GetDetail(ctx context.Context, id string) (*PaymentDetail, error) {
	paymentID, err := canonicalUUID(id)
	if err != nil {
		return nil, domain.ErrInvalidInput
	}
	tenantID, _, _ := tenantActor(ctx)
	filter := scope.FromActor(ctx, tenantID)
	payment, err := s.payments.Get(ctx, filter, tenantID, paymentID)
	if err != nil {
		return nil, err
	}
	allocations, err := s.payments.ListAllocations(ctx, filter, tenantID, paymentID)
	if err != nil {
		return nil, err
	}
	reversals, err := s.payments.ListReversals(ctx, filter, tenantID, paymentID)
	if err != nil {
		return nil, err
	}
	return &PaymentDetail{
		Payment:     payment,
		Allocations: allocations,
		Reversals:   reversals,
	}, nil
}

func (s *PaymentService) Unallocate(ctx context.Context, id string, input domain.UnallocateInput) (*PaymentDetail, error) {
	tenantID, _, userID := tenantActor(ctx)
	paymentID, err := canonicalUUID(id)
	if err != nil {
		return nil, domain.ErrInvalidInput
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return nil, domain.ErrReasonRequired
	}
	if len(input.InvoiceIDs) == 0 {
		return nil, domain.ErrInvalidInput
	}

	seen := make(map[string]struct{}, len(input.InvoiceIDs))
	invoiceIDs := make([]string, 0, len(input.InvoiceIDs))
	for _, raw := range input.InvoiceIDs {
		invoiceID, err := canonicalUUID(raw)
		if err != nil {
			return nil, domain.ErrInvalidInput
		}
		if _, exists := seen[invoiceID]; exists {
			return nil, domain.ErrDuplicateInvoice
		}
		seen[invoiceID] = struct{}{}
		invoiceIDs = append(invoiceIDs, invoiceID)
	}

	reversalDate := input.ReversalDate
	if reversalDate.IsZero() {
		reversalDate = time.Now()
	}
	reversalDate = normalizePaymentDate(reversalDate)

	filter := scope.FromActor(ctx, tenantID)
	payment, err := s.payments.Get(ctx, filter, tenantID, paymentID)
	if err != nil {
		return nil, err
	}
	if reversalDate.Before(normalizePaymentDate(payment.PaymentDate)) {
		return nil, domain.ErrInvalidInput
	}

	if _, err := s.payments.Unallocate(ctx, filter, tenantID, paymentID, userID, domain.UnallocateInput{
		InvoiceIDs:   invoiceIDs,
		ReversalDate: reversalDate,
		Reason:       reason,
	}); err != nil {
		return nil, err
	}
	return s.GetDetail(ctx, paymentID)
}

func (s *PaymentService) Create(ctx context.Context, input PaymentInput) (*domain.Payment, error) {
	tenantID, orgID, userID := tenantActor(ctx)

	paymentType := strings.ToLower(strings.TrimSpace(input.PaymentType))
	if paymentType != domain.PaymentTypeReceipt && paymentType != domain.PaymentTypePayment {
		return nil, domain.ErrInvalidInput
	}
	amount := input.Amount.Round(4)
	if !amount.IsPositive() {
		return nil, domain.ErrInvalidInput
	}

	filter := scope.FromActor(ctx, tenantID)
	partner, err := s.partners.FindByID(ctx, filter, tenantID, strings.TrimSpace(input.PartnerID))
	if err != nil || partner.Status != masterdomain.StatusActive {
		return nil, domain.ErrInvalidInput
	}
	if paymentType == domain.PaymentTypeReceipt &&
		partner.Type != masterdomain.PartnerTypeCustomer && partner.Type != masterdomain.PartnerTypeBoth {
		return nil, domain.ErrInvalidInput
	}
	if paymentType == domain.PaymentTypePayment &&
		partner.Type != masterdomain.PartnerTypeSupplier && partner.Type != masterdomain.PartnerTypeBoth {
		return nil, domain.ErrInvalidInput
	}

	entity := "finance_receipt"
	if paymentType == domain.PaymentTypePayment {
		entity = "finance_payment"
	}
	code, err := s.codes.Next(ctx, tenantID, entity)
	if err != nil {
		return nil, err
	}
	voucherCode, err := s.codes.Next(ctx, tenantID, "gl_voucher")
	if err != nil {
		return nil, err
	}

	paymentDate := input.PaymentDate
	if paymentDate.IsZero() {
		paymentDate = time.Now()
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = "CNY"
	}
	exchangeRate, err := fx.Resolve(ctx, s.rates, tenantID, currency, paymentDate, input.ExchangeRate)
	if err != nil {
		return nil, err
	}
	baseAmount, err := fx.Convert(amount, exchangeRate)
	if err != nil {
		return nil, err
	}
	method := strings.ToLower(strings.TrimSpace(input.Method))
	if method == "" {
		method = "bank"
	}
	if method != "bank" && method != "cash" && method != "other" {
		return nil, domain.ErrInvalidInput
	}

	// Payments belong to the acting organization. A cross-org settlement is
	// visible to users whose data scope covers that org (for example all/admin);
	// finance users with company/dept scope should settle invoices in their scope.
	payment := &domain.Payment{
		ID:                  uuid.NewString(),
		TenantID:            tenantID,
		OrgID:               orgID,
		Code:                code,
		PaymentType:         paymentType,
		PartnerID:           partner.ID,
		PartnerName:         partner.Name,
		PaymentDate:         paymentDate,
		Currency:            currency,
		ExchangeRate:        exchangeRate,
		Amount:              amount,
		AllocatedAmount:     decimal.Zero,
		BaseAmount:          baseAmount,
		BaseAllocatedAmount: decimal.Zero,
		Status:              domain.PaymentStatusUnallocated,
		Method:              method,
		Reference:           strings.TrimSpace(input.Reference),
		Remark:              strings.TrimSpace(input.Remark),
		CreatedBy:           userID,
		CreatedAt:           time.Now(),
		VoucherCode:         voucherCode,
	}
	if err := s.payments.Create(ctx, payment); err != nil {
		return nil, err
	}
	return payment, nil
}

func (s *PaymentService) Allocate(ctx context.Context, paymentID string, input AllocateInput) (*domain.Payment, error) {
	tenantID, _, userID := tenantActor(ctx)
	if len(input.Allocations) == 0 {
		return nil, domain.ErrInvalidInput
	}

	seen := make(map[string]struct{}, len(input.Allocations))
	allocations := make([]*domain.AllocationInput, 0, len(input.Allocations))
	for _, item := range input.Allocations {
		invoiceID := strings.TrimSpace(item.InvoiceID)
		amount := item.Amount.Round(4)
		if invoiceID == "" || !amount.IsPositive() {
			return nil, domain.ErrInvalidInput
		}
		parsed, err := uuid.Parse(invoiceID)
		if err != nil {
			return nil, domain.ErrInvalidInput
		}
		invoiceID = parsed.String()
		if _, exists := seen[invoiceID]; exists {
			return nil, domain.ErrInvalidInput
		}
		seen[invoiceID] = struct{}{}
		allocations = append(allocations, &domain.AllocationInput{
			InvoiceID: invoiceID,
			Amount:    amount,
		})
	}

	filter := scope.FromActor(ctx, tenantID)
	if err := s.payments.Allocate(ctx, filter, tenantID, paymentID, userID, allocations); err != nil {
		return nil, err
	}
	return s.payments.Get(ctx, filter, tenantID, paymentID)
}

func canonicalUUID(raw string) (string, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

func normalizePaymentDate(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}
