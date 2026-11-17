package saleshttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gldomain "github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
	"github.com/mlf-erp/mlf-erp/internal/modules/sales/app"
	"github.com/mlf-erp/mlf-erp/internal/modules/sales/domain"
	systemapp "github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
	"github.com/shopspring/decimal"
)

type Handler struct {
	orders    *app.OrderService
	shipments *app.ShipmentService
	audit     *systemapp.AuditService
	log       *slog.Logger
}

func Register(
	group *gin.RouterGroup,
	orders *app.OrderService,
	shipments *app.ShipmentService,
	audit *systemapp.AuditService,
	log *slog.Logger,
) {
	handler := &Handler{orders: orders, shipments: shipments, audit: audit, log: log}
	sales := group.Group("/sales")
	sales.GET("/orders", authmw.RequirePermission("sales.order.read"), handler.listOrders)
	sales.POST("/orders", authmw.RequirePermission("sales.order.write"), handler.createOrder)
	sales.GET("/orders/:id", authmw.RequirePermission("sales.order.read"), handler.getOrder)
	sales.PUT("/orders/:id", authmw.RequirePermission("sales.order.write"), handler.updateOrder)
	sales.POST("/orders/:id/submit", authmw.RequirePermission("sales.order.write"), handler.submitOrder)
	sales.POST("/orders/:id/approve", authmw.RequirePermission("sales.order.approve"), handler.approveOrder)
	sales.POST("/orders/:id/cancel", authmw.RequirePermission("sales.order.write"), handler.cancelOrder)
	sales.POST("/orders/:id/ship", authmw.RequirePermission("sales.order.write"), handler.shipOrder)
	sales.GET("/shipments", authmw.RequirePermission("sales.shipment.read"), handler.listShipments)
}

type orderLineRequest struct {
	MaterialID string `json:"material_id"`
	Quantity   string `json:"quantity"`
	UnitPrice  string `json:"unit_price"`
}

type orderRequest struct {
	CustomerID  string             `json:"customer_id"`
	WarehouseID string             `json:"warehouse_id"`
	OrderDate   string             `json:"order_date"`
	Currency    string             `json:"currency"`
	Remark      string             `json:"remark"`
	Lines       []orderLineRequest `json:"lines"`
}

type shipLineRequest struct {
	OrderLineID string `json:"order_line_id"`
	Quantity    string `json:"quantity"`
}

type shipRequest struct {
	ShipmentDate string            `json:"shipment_date"`
	Remark       string            `json:"remark"`
	Lines        []shipLineRequest `json:"lines"`
}

type orderLineResponse struct {
	ID           string `json:"id"`
	LineNo       int    `json:"line_no"`
	MaterialID   string `json:"material_id"`
	MaterialName string `json:"material_name"`
	Quantity     string `json:"quantity"`
	UnitPrice    string `json:"unit_price"`
	Amount       string `json:"amount"`
	ShippedQty   string `json:"shipped_qty"`
	RemainingQty string `json:"remaining_qty"`
}

type orderResponse struct {
	ID            string              `json:"id"`
	Code          string              `json:"code"`
	CustomerID    string              `json:"customer_id"`
	CustomerName  string              `json:"customer_name"`
	WarehouseID   string              `json:"warehouse_id"`
	WarehouseName string              `json:"warehouse_name"`
	OrderDate     string              `json:"order_date"`
	Currency      string              `json:"currency"`
	Status        string              `json:"status"`
	TotalAmount   string              `json:"total_amount"`
	Remark        string              `json:"remark"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
	Lines         []orderLineResponse `json:"lines"`
}

type shipmentLineResponse struct {
	OrderLineID  string `json:"order_line_id"`
	MaterialID   string `json:"material_id"`
	MaterialName string `json:"material_name"`
	Quantity     string `json:"quantity"`
	UnitPrice    string `json:"unit_price"`
	Amount       string `json:"amount"`
	UnitCost     string `json:"unit_cost"`
	CostAmount   string `json:"cost_amount"`
}

type shipmentResponse struct {
	ID             string                 `json:"id"`
	Code           string                 `json:"code"`
	OrderID        string                 `json:"order_id"`
	OrderCode      string                 `json:"order_code"`
	CustomerID     string                 `json:"customer_id"`
	CustomerName   string                 `json:"customer_name"`
	WarehouseID    string                 `json:"warehouse_id"`
	WarehouseName  string                 `json:"warehouse_name"`
	ShipmentDate   string                 `json:"shipment_date"`
	Status         string                 `json:"status"`
	Remark         string                 `json:"remark"`
	InvoiceID      string                 `json:"invoice_id"`
	InvoiceCode    string                 `json:"invoice_code"`
	InvoiceDueDate string                 `json:"invoice_due_date"`
	TotalAmount    string                 `json:"total_amount"`
	TotalCost      string                 `json:"total_cost"`
	CreatedAt      time.Time              `json:"created_at"`
	Lines          []shipmentLineResponse `json:"lines"`
}

type pagedResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

func (h *Handler) listOrders(c *gin.Context) {
	page, pageSize := parsePage(c)
	orders, total, err := h.orders.List(c.Request.Context(), c.Query("status"), c.Query("keyword"), page, pageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}

	items := make([]orderResponse, 0, len(orders))
	for _, order := range orders {
		items = append(items, toOrderResponse(order))
	}
	httpx.OK(c, pagedResponse[orderResponse]{Items: items, Total: total})
}

func (h *Handler) getOrder(c *gin.Context) {
	order, err := h.orders.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	httpx.OK(c, toOrderResponse(order))
}

func (h *Handler) createOrder(c *gin.Context) {
	input, err := parseOrderInput(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid sales order payload")
		return
	}
	order, err := h.orders.Create(c.Request.Context(), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toOrderResponse(order)
	h.recordAudit(c, "sales.order.create", "sales_order", order.ID, response)
	httpx.Created(c, response)
}

func (h *Handler) updateOrder(c *gin.Context) {
	input, err := parseOrderInput(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid sales order payload")
		return
	}
	order, err := h.orders.UpdateDraft(c.Request.Context(), c.Param("id"), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toOrderResponse(order)
	h.recordAudit(c, "sales.order.update", "sales_order", order.ID, response)
	httpx.OK(c, response)
}

func (h *Handler) submitOrder(c *gin.Context) {
	h.transition(c, "sales.order.submit", h.orders.Submit)
}

func (h *Handler) approveOrder(c *gin.Context) {
	h.transition(c, "sales.order.approve", h.orders.Approve)
}

func (h *Handler) cancelOrder(c *gin.Context) {
	h.transition(c, "sales.order.cancel", h.orders.Cancel)
}

func (h *Handler) transition(c *gin.Context, action string, fn func(ctx context.Context, id string) error) {
	if err := fn(c.Request.Context(), c.Param("id")); err != nil {
		h.handleError(c, err)
		return
	}
	h.recordAudit(c, action, "sales_order", c.Param("id"), nil)
	httpx.OK(c, gin.H{"id": c.Param("id"), "action": action})
}

func (h *Handler) shipOrder(c *gin.Context) {
	var request shipRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid ship payload")
		return
	}
	input, err := parseShipInput(request)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid ship payload")
		return
	}
	shipment, err := h.shipments.Ship(c.Request.Context(), c.Param("id"), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toShipmentResponse(shipment)
	h.recordAudit(c, "sales.shipment.create", "sales_shipment", shipment.ID, response)
	httpx.Created(c, response)
}

func (h *Handler) listShipments(c *gin.Context) {
	page, pageSize := parsePage(c)
	shipments, total, err := h.shipments.List(c.Request.Context(), c.Query("order_id"), page, pageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}

	items := make([]shipmentResponse, 0, len(shipments))
	for _, shipment := range shipments {
		items = append(items, toShipmentResponse(shipment))
	}
	httpx.OK(c, pagedResponse[shipmentResponse]{Items: items, Total: total})
}

func parseOrderInput(c *gin.Context) (app.OrderInput, error) {
	var request orderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		return app.OrderInput{}, err
	}
	orderDate, err := parseDate(request.OrderDate)
	if err != nil {
		return app.OrderInput{}, err
	}
	lines := make([]app.OrderLineInput, 0, len(request.Lines))
	for _, line := range request.Lines {
		quantity, quantityErr := decimal.NewFromString(strings.TrimSpace(line.Quantity))
		if quantityErr != nil {
			return app.OrderInput{}, quantityErr
		}
		unitPrice, priceErr := decimal.NewFromString(strings.TrimSpace(line.UnitPrice))
		if priceErr != nil {
			return app.OrderInput{}, priceErr
		}
		lines = append(lines, app.OrderLineInput{
			MaterialID: line.MaterialID,
			Quantity:   quantity,
			UnitPrice:  unitPrice,
		})
	}
	return app.OrderInput{
		CustomerID: request.CustomerID, WarehouseID: request.WarehouseID,
		OrderDate: orderDate, Currency: request.Currency, Remark: request.Remark, Lines: lines,
	}, nil
}

func parseShipInput(request shipRequest) (app.ShipInput, error) {
	shipmentDate, err := parseDate(request.ShipmentDate)
	if err != nil {
		return app.ShipInput{}, err
	}
	lines := make([]app.ShipLineInput, 0, len(request.Lines))
	for _, line := range request.Lines {
		quantity, quantityErr := decimal.NewFromString(strings.TrimSpace(line.Quantity))
		if quantityErr != nil {
			return app.ShipInput{}, quantityErr
		}
		lines = append(lines, app.ShipLineInput{OrderLineID: line.OrderLineID, Quantity: quantity})
	}
	return app.ShipInput{ShipmentDate: shipmentDate, Remark: request.Remark, Lines: lines}, nil
}

func parseDate(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", value)
}

func parsePage(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.Error(c, http.StatusNotFound, httpx.CodeNotFound, "resource not found")
	case errors.Is(err, domain.ErrInvalidInput):
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid input")
	case errors.Is(err, domain.ErrOverShipment):
		httpx.Error(c, http.StatusConflict, "OVER_SHIPMENT", "shipped quantity exceeds remaining quantity")
	case errors.Is(err, domain.ErrInsufficientStock):
		httpx.Error(c, http.StatusConflict, "INSUFFICIENT_STOCK", "insufficient inventory")
	case errors.Is(err, domain.ErrInvalidStatus):
		httpx.Error(c, http.StatusConflict, "INVALID_STATUS", "order status does not allow this operation")
	case errors.Is(err, domain.ErrConflict):
		httpx.Error(c, http.StatusConflict, "CONFLICT", "sales order or shipment is in conflict")
	case errors.Is(err, gldomain.ErrConflict):
		httpx.Error(c, http.StatusConflict, "GL_CONFLICT", "general ledger posting is in conflict")
	case errors.Is(err, gldomain.ErrPeriodClosed):
		httpx.Error(c, http.StatusConflict, "PERIOD_CLOSED", "accounting period is closed")
	case errors.Is(err, gldomain.ErrAccountNotLeaf):
		httpx.Error(c, http.StatusBadRequest, "ACCOUNT_NOT_LEAF", "account must be a leaf")
	case errors.Is(err, gldomain.ErrAccountDisabled):
		httpx.Error(c, http.StatusBadRequest, "ACCOUNT_DISABLED", "account is disabled")
	case errors.Is(err, gldomain.ErrUnbalanced):
		httpx.Error(c, http.StatusBadRequest, "UNBALANCED", "automatic voucher is not balanced")
	case errors.Is(err, gldomain.ErrInvalidInput):
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "general ledger posting is not configured")
	default:
		h.log.Error("sales request failed",
			"error", err,
			"request_id", c.GetString(httpmw.RequestIDKey),
			"path", c.Request.URL.Path,
		)
		httpx.Error(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
	}
}

func toOrderResponse(order *domain.Order) orderResponse {
	lines := make([]orderLineResponse, 0, len(order.Lines))
	for _, line := range order.Lines {
		lines = append(lines, orderLineResponse{
			ID: line.ID, LineNo: line.LineNo, MaterialID: line.MaterialID, MaterialName: line.MaterialName,
			Quantity: line.Quantity.String(), UnitPrice: line.UnitPrice.String(),
			Amount: line.Amount.StringFixed(4), ShippedQty: line.ShippedQty.String(),
			RemainingQty: line.Remaining().String(),
		})
	}
	return orderResponse{
		ID: order.ID, Code: order.Code, CustomerID: order.CustomerID, CustomerName: order.CustomerName,
		WarehouseID: order.WarehouseID, WarehouseName: order.WarehouseName,
		OrderDate: order.OrderDate.Format("2006-01-02"), Currency: order.Currency,
		Status: string(order.Status), TotalAmount: order.TotalAmount.StringFixed(4),
		Remark: order.Remark, CreatedAt: order.CreatedAt, UpdatedAt: order.UpdatedAt, Lines: lines,
	}
}

func toShipmentResponse(shipment *domain.Shipment) shipmentResponse {
	lines := make([]shipmentLineResponse, 0, len(shipment.Lines))
	for _, line := range shipment.Lines {
		lines = append(lines, shipmentLineResponse{
			OrderLineID: line.OrderLineID, MaterialID: line.MaterialID, MaterialName: line.MaterialName,
			Quantity: line.Quantity.String(), UnitPrice: line.UnitPrice.String(),
			Amount: line.Amount.StringFixed(4), UnitCost: line.UnitCost.String(),
			CostAmount: line.CostAmount.StringFixed(4),
		})
	}
	invoiceDueDate := ""
	if !shipment.InvoiceDueDate.IsZero() {
		invoiceDueDate = shipment.InvoiceDueDate.Format("2006-01-02")
	}
	return shipmentResponse{
		ID: shipment.ID, Code: shipment.Code, OrderID: shipment.OrderID, OrderCode: shipment.OrderCode,
		CustomerID: shipment.CustomerID, CustomerName: shipment.CustomerName,
		WarehouseID: shipment.WarehouseID, WarehouseName: shipment.WarehouseName,
		ShipmentDate: shipment.ShipmentDate.Format("2006-01-02"), Status: shipment.Status,
		Remark: shipment.Remark, InvoiceID: shipment.InvoiceID, InvoiceCode: shipment.InvoiceCode,
		InvoiceDueDate: invoiceDueDate,
		TotalAmount:    shipment.TotalAmount().StringFixed(4), TotalCost: shipment.TotalCost().StringFixed(4),
		CreatedAt: shipment.CreatedAt, Lines: lines,
	}
}

func (h *Handler) recordAudit(c *gin.Context, action, resourceType, resourceID string, after any) {
	if h.audit == nil {
		return
	}
	_ = h.audit.Record(c.Request.Context(), systemapp.AuditInput{
		Action: action, ResourceType: resourceType, ResourceID: resourceID,
		RequestID: c.GetString(httpmw.RequestIDKey), IP: c.ClientIP(), UserAgent: c.Request.UserAgent(),
		After: after,
	})
}
