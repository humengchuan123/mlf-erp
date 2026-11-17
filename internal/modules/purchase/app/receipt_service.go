package app

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/purchase/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type ReceiptService struct {
	orders domain.OrderRepository
	codes  CodeGenerator
}

func NewReceiptService(orders domain.OrderRepository, codes CodeGenerator) *ReceiptService {
	return &ReceiptService{orders: orders, codes: codes}
}

type ReceiveLineInput struct {
	OrderLineID string
	Quantity    decimal.Decimal
}

type ReceiveInput struct {
	ReceiptDate time.Time
	Remark      string
	Lines       []ReceiveLineInput
}

func (s *ReceiptService) Receive(ctx context.Context, orderID string, input ReceiveInput) (*domain.Receipt, error) {
	tenantID, _, userID := tenantActor(ctx)
	filter := scope.FromActor(ctx, tenantID)
	order, err := s.orders.Get(ctx, filter, tenantID, orderID)
	if err != nil {
		return nil, err
	}
	if !order.Status.CanReceive() {
		return nil, domain.ErrInvalidStatus
	}
	if len(input.Lines) == 0 {
		return nil, domain.ErrInvalidInput
	}

	lineByID := make(map[string]*domain.OrderLine, len(order.Lines))
	for _, line := range order.Lines {
		lineByID[line.ID] = line
	}

	requested := make(map[string]decimal.Decimal, len(input.Lines))
	for _, inputLine := range input.Lines {
		if !inputLine.Quantity.IsPositive() {
			return nil, domain.ErrInvalidInput
		}
		if _, ok := lineByID[inputLine.OrderLineID]; !ok {
			return nil, domain.ErrInvalidInput
		}
		requested[inputLine.OrderLineID] = requested[inputLine.OrderLineID].Add(inputLine.Quantity)
	}

	lineIDs := make([]string, 0, len(requested))
	for lineID := range requested {
		lineIDs = append(lineIDs, lineID)
	}
	sort.Strings(lineIDs)

	receiptID := uuid.NewString()
	receiptLines := make([]*domain.ReceiptLine, 0, len(lineIDs))
	for _, lineID := range lineIDs {
		orderLine := lineByID[lineID]
		quantity := requested[lineID]
		if quantity.GreaterThan(orderLine.Remaining()) {
			return nil, domain.ErrOverReceive
		}
		receiptLines = append(receiptLines, &domain.ReceiptLine{
			ID:           uuid.NewString(),
			TenantID:     tenantID,
			ReceiptID:    receiptID,
			OrderLineID:  orderLine.ID,
			MaterialID:   orderLine.MaterialID,
			MaterialName: orderLine.MaterialName,
			Quantity:     quantity,
			UnitPrice:    orderLine.UnitPrice,
			Amount:       quantity.Mul(orderLine.UnitPrice).Round(4),
		})
	}

	code, err := s.codes.Next(ctx, tenantID, "purchase_receipt")
	if err != nil {
		return nil, err
	}
	payableCode, err := s.codes.Next(ctx, tenantID, "purchase_payable")
	if err != nil {
		return nil, err
	}
	receiptDate := input.ReceiptDate
	if receiptDate.IsZero() {
		receiptDate = time.Now()
	}

	receipt := &domain.Receipt{
		ID:             receiptID,
		TenantID:       tenantID,
		OrgID:          order.OrgID,
		Code:           code,
		OrderID:        order.ID,
		OrderCode:      order.Code,
		SupplierName:   order.SupplierName,
		WarehouseID:    order.WarehouseID,
		WarehouseName:  order.WarehouseName,
		ReceiptDate:    receiptDate,
		Status:         "posted",
		Remark:         strings.TrimSpace(input.Remark),
		CreatedBy:      userID,
		PayableID:      uuid.NewString(),
		PayableCode:    payableCode,
		PayableDueDate: receiptDate.AddDate(0, 0, 30),
		Lines:          receiptLines,
	}

	if err := s.orders.Receive(ctx, filter, receipt); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (s *ReceiptService) List(ctx context.Context, orderID string, page, pageSize int) ([]*domain.Receipt, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.orders.ListReceipts(ctx, scope.FromActor(ctx, tenantID), tenantID, orderID, (page-1)*pageSize, pageSize)
}
