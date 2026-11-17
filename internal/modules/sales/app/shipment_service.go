package app

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/sales/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type ShipmentService struct {
	orders domain.OrderRepository
	codes  CodeGenerator
}

func NewShipmentService(orders domain.OrderRepository, codes CodeGenerator) *ShipmentService {
	return &ShipmentService{orders: orders, codes: codes}
}

type ShipLineInput struct {
	OrderLineID string
	Quantity    decimal.Decimal
}

type ShipInput struct {
	ShipmentDate time.Time
	Remark       string
	Lines        []ShipLineInput
}

func (s *ShipmentService) Ship(ctx context.Context, orderID string, input ShipInput) (*domain.Shipment, error) {
	tenantID, _, userID := tenantActor(ctx)
	filter := scope.FromActor(ctx, tenantID)
	order, err := s.orders.Get(ctx, filter, tenantID, orderID)
	if err != nil {
		return nil, err
	}
	if !order.Status.CanShip() {
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

	shipmentID := uuid.NewString()
	shipmentLines := make([]*domain.ShipmentLine, 0, len(lineIDs))
	for _, lineID := range lineIDs {
		orderLine := lineByID[lineID]
		quantity := requested[lineID]
		if quantity.GreaterThan(orderLine.Remaining()) {
			return nil, domain.ErrOverShipment
		}
		shipmentLines = append(shipmentLines, &domain.ShipmentLine{
			ID:           uuid.NewString(),
			TenantID:     tenantID,
			ShipmentID:   shipmentID,
			OrderLineID:  orderLine.ID,
			MaterialID:   orderLine.MaterialID,
			MaterialName: orderLine.MaterialName,
			Quantity:     quantity,
			UnitPrice:    orderLine.UnitPrice,
			Amount:       quantity.Mul(orderLine.UnitPrice).Round(4),
		})
	}

	code, err := s.codes.Next(ctx, tenantID, "sales_shipment")
	if err != nil {
		return nil, err
	}
	invoiceCode, err := s.codes.Next(ctx, tenantID, "sales_receivable")
	if err != nil {
		return nil, err
	}
	shipmentDate := input.ShipmentDate
	if shipmentDate.IsZero() {
		shipmentDate = time.Now()
	}

	shipment := &domain.Shipment{
		ID:             shipmentID,
		TenantID:       tenantID,
		OrgID:          order.OrgID,
		Code:           code,
		OrderID:        order.ID,
		OrderCode:      order.Code,
		CustomerID:     order.CustomerID,
		CustomerName:   order.CustomerName,
		WarehouseID:    order.WarehouseID,
		WarehouseName:  order.WarehouseName,
		ShipmentDate:   shipmentDate,
		Status:         "posted",
		Remark:         strings.TrimSpace(input.Remark),
		CreatedBy:      userID,
		CreatedAt:      time.Now(),
		InvoiceID:      uuid.NewString(),
		InvoiceCode:    invoiceCode,
		InvoiceDueDate: shipmentDate.AddDate(0, 0, 30),
		Lines:          shipmentLines,
	}

	if err := s.orders.Ship(ctx, filter, shipment); err != nil {
		return nil, err
	}
	return shipment, nil
}

func (s *ShipmentService) List(ctx context.Context, orderID string, page, pageSize int) ([]*domain.Shipment, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.orders.ListShipments(ctx, scope.FromActor(ctx, tenantID), tenantID, orderID, (page-1)*pageSize, pageSize)
}
