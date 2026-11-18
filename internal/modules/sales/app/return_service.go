package app

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/sales/domain"
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

func (s *ReturnService) Create(ctx context.Context, shipmentID string, input ReturnInput) (*domain.SalesReturn, error) {
	tenantID, _, userID := tenantActor(ctx)
	shipmentID = strings.TrimSpace(shipmentID)
	if shipmentID == "" {
		return nil, domain.ErrInvalidInput
	}
	if _, err := uuid.Parse(shipmentID); err != nil {
		return nil, domain.ErrInvalidInput
	}

	returnCode, err := s.codes.Next(ctx, tenantID, "sales_return")
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
		ShipmentID:     shipmentID,
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
	shipmentID, customerID, keyword string,
	startDate, endDate time.Time,
	page, pageSize int,
) ([]*domain.SalesReturn, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	shipmentID = strings.TrimSpace(shipmentID)
	customerID = strings.TrimSpace(customerID)
	if shipmentID != "" {
		if _, err := uuid.Parse(shipmentID); err != nil {
			return nil, 0, domain.ErrInvalidInput
		}
	}
	if customerID != "" {
		if _, err := uuid.Parse(customerID); err != nil {
			return nil, 0, domain.ErrInvalidInput
		}
	}
	return s.returns.List(
		ctx,
		scope.FromActor(ctx, tenantID),
		tenantID,
		shipmentID,
		customerID,
		strings.TrimSpace(keyword),
		startDate,
		endDate,
		(page-1)*pageSize,
		pageSize,
	)
}

func (s *ReturnService) Get(ctx context.Context, id string) (*domain.SalesReturn, error) {
	tenantID, _, _ := tenantActor(ctx)
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return s.returns.Get(ctx, scope.FromActor(ctx, tenantID), tenantID, id)
}
