package app

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/purchase/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type ReturnService struct {
	returns domain.ReturnRepository
	codes   CodeGenerator
}

func NewReturnService(returns domain.ReturnRepository, codes CodeGenerator) *ReturnService {
	return &ReturnService{returns: returns, codes: codes}
}

type ReturnInput struct {
	ReturnDate time.Time
	Reason     string
	Remark     string
	Lines      []domain.ReturnLineInput
}

func (s *ReturnService) Create(ctx context.Context, receiptID string, input ReturnInput) (*domain.PurchaseReturn, error) {
	tenantID, _, userID := tenantActor(ctx)
	receiptID = strings.TrimSpace(receiptID)
	if receiptID == "" {
		return nil, domain.ErrInvalidInput
	}
	if _, err := uuid.Parse(receiptID); err != nil {
		return nil, domain.ErrInvalidInput
	}

	returnCode, err := s.codes.Next(ctx, tenantID, "purchase_return")
	if err != nil {
		return nil, err
	}
	creditNoteCode, err := s.codes.Next(ctx, tenantID, "finance_credit_note")
	if err != nil {
		return nil, err
	}
	voucherCode, err := s.codes.Next(ctx, tenantID, "gl_voucher")
	if err != nil {
		return nil, err
	}

	returnDate := input.ReturnDate
	if returnDate.IsZero() {
		returnDate = time.Now()
	}

	return s.returns.Create(ctx, scope.FromActor(ctx, tenantID), tenantID, userID, domain.ReturnCreateParams{
		ReceiptID:      receiptID,
		ReturnDate:     returnDate,
		Reason:         strings.TrimSpace(input.Reason),
		Remark:         strings.TrimSpace(input.Remark),
		Lines:          input.Lines,
		ReturnCode:     returnCode,
		CreditNoteCode: creditNoteCode,
		VoucherCode:    voucherCode,
	})
}

func (s *ReturnService) List(
	ctx context.Context,
	receiptID, supplierID, keyword string,
	startDate, endDate time.Time,
	page, pageSize int,
) ([]*domain.PurchaseReturn, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	receiptID = strings.TrimSpace(receiptID)
	supplierID = strings.TrimSpace(supplierID)
	if receiptID != "" {
		if _, err := uuid.Parse(receiptID); err != nil {
			return nil, 0, domain.ErrInvalidInput
		}
	}
	if supplierID != "" {
		if _, err := uuid.Parse(supplierID); err != nil {
			return nil, 0, domain.ErrInvalidInput
		}
	}
	return s.returns.List(
		ctx,
		scope.FromActor(ctx, tenantID),
		tenantID,
		receiptID,
		supplierID,
		strings.TrimSpace(keyword),
		startDate,
		endDate,
		(page-1)*pageSize,
		pageSize,
	)
}

func (s *ReturnService) Get(ctx context.Context, id string) (*domain.PurchaseReturn, error) {
	tenantID, _, _ := tenantActor(ctx)
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return s.returns.Get(ctx, scope.FromActor(ctx, tenantID), tenantID, id)
}
