package app

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	masterdomain "github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	"github.com/mlf-erp/mlf-erp/internal/modules/sales/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type OrderService struct {
	orders     domain.OrderRepository
	codes      CodeGenerator
	partners   PartnerReader
	materials  MaterialReader
	warehouses WarehouseReader
}

func NewOrderService(
	orders domain.OrderRepository,
	codes CodeGenerator,
	partners PartnerReader,
	materials MaterialReader,
	warehouses WarehouseReader,
) *OrderService {
	return &OrderService{
		orders: orders, codes: codes, partners: partners, materials: materials, warehouses: warehouses,
	}
}

type OrderLineInput struct {
	MaterialID string
	Quantity   decimal.Decimal
	UnitPrice  decimal.Decimal
}

type OrderInput struct {
	CustomerID  string
	WarehouseID string
	OrderDate   time.Time
	Currency    string
	Remark      string
	Lines       []OrderLineInput
}

func (s *OrderService) List(ctx context.Context, status, keyword string, page, pageSize int) ([]*domain.Order, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.orders.List(ctx, scope.FromActor(ctx, tenantID), tenantID, status, strings.TrimSpace(keyword), (page-1)*pageSize, pageSize)
}

func (s *OrderService) Get(ctx context.Context, id string) (*domain.Order, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.orders.Get(ctx, scope.FromActor(ctx, tenantID), tenantID, id)
}

func (s *OrderService) Create(ctx context.Context, input OrderInput) (*domain.Order, error) {
	tenantID, _, userID := tenantActor(ctx)
	partner, warehouse, materials, err := s.validate(ctx, tenantID, input)
	if err != nil {
		return nil, err
	}

	code, err := s.codes.Next(ctx, tenantID, "sales_order")
	if err != nil {
		return nil, err
	}

	orderDate := input.OrderDate
	if orderDate.IsZero() {
		orderDate = time.Now()
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = "CNY"
	}

	order := &domain.Order{
		ID:            uuid.NewString(),
		TenantID:      tenantID,
		OrgID:         warehouse.OrgID,
		Code:          code,
		CustomerID:    partner.ID,
		CustomerName:  partner.Name,
		WarehouseID:   warehouse.ID,
		WarehouseName: warehouse.Name,
		OrderDate:     orderDate,
		Currency:      currency,
		Status:        domain.StatusDraft,
		Remark:        strings.TrimSpace(input.Remark),
		CreatedBy:     userID,
		UpdatedBy:     userID,
	}
	order.Lines = buildLines(order.ID, tenantID, input.Lines, materials)
	order.Recalculate()

	if err := s.orders.Create(ctx, order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) UpdateDraft(ctx context.Context, id string, input OrderInput) (*domain.Order, error) {
	tenantID, _, userID := tenantActor(ctx)
	filter := scope.FromActor(ctx, tenantID)
	order, err := s.orders.Get(ctx, filter, tenantID, id)
	if err != nil {
		return nil, err
	}
	if !order.Status.CanEdit() {
		return nil, domain.ErrInvalidStatus
	}

	partner, warehouse, materials, err := s.validate(ctx, tenantID, input)
	if err != nil {
		return nil, err
	}
	orderDate := input.OrderDate
	if orderDate.IsZero() {
		orderDate = order.OrderDate
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = order.Currency
	}

	order.OrgID = warehouse.OrgID
	order.CustomerID = partner.ID
	order.CustomerName = partner.Name
	order.WarehouseID = warehouse.ID
	order.WarehouseName = warehouse.Name
	order.OrderDate = orderDate
	order.Currency = currency
	order.Remark = strings.TrimSpace(input.Remark)
	order.UpdatedBy = userID
	order.Lines = buildLines(order.ID, tenantID, input.Lines, materials)
	order.Recalculate()

	if err := s.orders.UpdateDraft(ctx, filter, order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) Submit(ctx context.Context, id string) error {
	return s.transition(ctx, id, domain.StatusDraft, domain.StatusSubmitted)
}

func (s *OrderService) Approve(ctx context.Context, id string) error {
	return s.transition(ctx, id, domain.StatusSubmitted, domain.StatusApproved)
}

func (s *OrderService) Cancel(ctx context.Context, id string) error {
	tenantID, _, userID := tenantActor(ctx)
	order, err := s.orders.Get(ctx, scope.FromActor(ctx, tenantID), tenantID, id)
	if err != nil {
		return err
	}
	if !order.Status.CanCancel() {
		return domain.ErrInvalidStatus
	}
	for _, line := range order.Lines {
		if line.ShippedQty.IsPositive() {
			return domain.ErrInvalidStatus
		}
	}
	ok, err := s.orders.UpdateStatus(ctx, tenantID, id, order.Status, domain.StatusCancelled, userID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrConflict
	}
	return nil
}

func (s *OrderService) transition(ctx context.Context, id string, from, to domain.Status) error {
	tenantID, _, userID := tenantActor(ctx)
	order, err := s.orders.Get(ctx, scope.FromActor(ctx, tenantID), tenantID, id)
	if err != nil {
		return err
	}
	if order.Status != from {
		return domain.ErrInvalidStatus
	}
	ok, err := s.orders.UpdateStatus(ctx, tenantID, id, from, to, userID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrConflict
	}
	return nil
}

func (s *OrderService) validate(
	ctx context.Context,
	tenantID string,
	input OrderInput,
) (*masterdomain.Partner, *masterdomain.Warehouse, []masterdomain.Material, error) {
	filter := scope.FromActor(ctx, tenantID)

	partner, err := s.partners.FindByID(ctx, filter, tenantID, input.CustomerID)
	if err != nil || partner.Status != masterdomain.StatusActive {
		return nil, nil, nil, domain.ErrInvalidInput
	}
	if partner.Type != masterdomain.PartnerTypeCustomer && partner.Type != masterdomain.PartnerTypeBoth {
		return nil, nil, nil, domain.ErrInvalidInput
	}

	warehouse, err := s.warehouses.FindByID(ctx, filter, tenantID, input.WarehouseID)
	if err != nil || warehouse.Status != masterdomain.StatusActive {
		return nil, nil, nil, domain.ErrInvalidInput
	}

	if len(input.Lines) == 0 {
		return nil, nil, nil, domain.ErrInvalidInput
	}

	materials := make([]masterdomain.Material, 0, len(input.Lines))
	for _, line := range input.Lines {
		if !line.Quantity.IsPositive() || line.UnitPrice.IsNegative() {
			return nil, nil, nil, domain.ErrInvalidInput
		}
		material, materialErr := s.materials.FindByID(ctx, filter, tenantID, line.MaterialID)
		if materialErr != nil || material.Status != masterdomain.StatusActive {
			return nil, nil, nil, domain.ErrInvalidInput
		}
		materials = append(materials, material)
	}
	return &partner, &warehouse, materials, nil
}

func buildLines(orderID, tenantID string, inputs []OrderLineInput, materials []masterdomain.Material) []*domain.OrderLine {
	lines := make([]*domain.OrderLine, 0, len(inputs))
	for i, input := range inputs {
		lines = append(lines, &domain.OrderLine{
			ID:           uuid.NewString(),
			TenantID:     tenantID,
			OrderID:      orderID,
			LineNo:       i + 1,
			MaterialID:   input.MaterialID,
			MaterialName: materials[i].Name,
			Quantity:     input.Quantity,
			UnitPrice:    input.UnitPrice,
			Amount:       input.Quantity.Mul(input.UnitPrice).Round(4),
			ShippedQty:   decimal.Zero,
		})
	}
	return lines
}
