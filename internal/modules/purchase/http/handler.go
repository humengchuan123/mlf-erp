package purchasehttp

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
	"github.com/mlf-erp/mlf-erp/internal/modules/purchase/app"
	"github.com/mlf-erp/mlf-erp/internal/modules/purchase/domain"
	systemapp "github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
	"github.com/shopspring/decimal"
)

type Handler struct {
	orders   *app.OrderService
	receipts *app.ReceiptService
	audit    *systemapp.AuditService
	log      *slog.Logger
}

func Register(
	group *gin.RouterGroup,
	orders *app.OrderService,
	receipts *app.ReceiptService,
	audit *systemapp.AuditService,
	log *slog.Logger,
) {
	handler := &Handler{orders: orders, receipts: receipts, audit: audit, log: log}
	purchase := group.Group("/purchase")
	purchase.GET("/orders", authmw.RequirePermission("purchase.order.read"), handler.listOrders)
	purchase.POST("/orders", authmw.RequirePermission("purchase.order.write"), handler.createOrder)
	purchase.GET("/orders/:id", authmw.RequirePermission("purchase.order.read"), handler.getOrder)
	purchase.PUT("/orders/:id", authmw.RequirePermission("purchase.order.write"), handler.updateOrder)
	purchase.POST("/orders/:id/submit", authmw.RequirePermission("purchase.order.write"), handler.submitOrder)
	purchase.POST("/orders/:id/approve", authmw.RequirePermission("purchase.order.approve"), handler.approveOrder)
	purchase.POST("/orders/:id/cancel", authmw.RequirePermission("purchase.order.write"), handler.cancelOrder)
	purchase.POST("/orders/:id/receive", authmw.RequirePermission("purchase.order.write"), handler.receiveOrder)
	purchase.GET("/receipts", authmw.RequirePermission("purchase.receipt.read"), handler.listReceipts)
}

type orderLineRequest struct {
	MaterialID string `json:"material_id"`
	Quantity   string `json:"quantity"`
	UnitPrice  string `json:"unit_price"`
	TaxRateID  string `json:"tax_rate_id"`
}

type orderRequest struct {
	SupplierID       string             `json:"supplier_id"`
	WarehouseID      string             `json:"warehouse_id"`
	OrderDate        string             `json:"order_date"`
	Currency         string             `json:"currency"`
	ExchangeRate     string             `json:"exchange_rate"`
	PriceIncludesTax *bool              `json:"price_includes_tax"`
	Remark           string             `json:"remark"`
	Lines            []orderLineRequest `json:"lines"`
}

type receiveLineRequest struct {
	OrderLineID string `json:"order_line_id"`
	Quantity    string `json:"quantity"`
}

type receiveRequest struct {
	ReceiptDate  string               `json:"receipt_date"`
	ExchangeRate string               `json:"exchange_rate"`
	Remark       string               `json:"remark"`
	Lines        []receiveLineRequest `json:"lines"`
}

type orderLineResponse struct {
	ID              string `json:"id"`
	LineNo          int    `json:"line_no"`
	MaterialID      string `json:"material_id"`
	MaterialName    string `json:"material_name"`
	Quantity        string `json:"quantity"`
	UnitPrice       string `json:"unit_price"`
	TaxRateID       string `json:"tax_rate_id"`
	TaxRate         string `json:"tax_rate"`
	Amount          string `json:"amount"`
	TaxAmount       string `json:"tax_amount"`
	GrossAmount     string `json:"gross_amount"`
	BaseAmount      string `json:"base_amount"`
	BaseTaxAmount   string `json:"base_tax_amount"`
	BaseGrossAmount string `json:"base_gross_amount"`
	ReceivedQty     string `json:"received_qty"`
	RemainingQty    string `json:"remaining_qty"`
}

type orderResponse struct {
	ID               string              `json:"id"`
	Code             string              `json:"code"`
	SupplierID       string              `json:"supplier_id"`
	SupplierName     string              `json:"supplier_name"`
	WarehouseID      string              `json:"warehouse_id"`
	WarehouseName    string              `json:"warehouse_name"`
	OrderDate        string              `json:"order_date"`
	Currency         string              `json:"currency"`
	ExchangeRate     string              `json:"exchange_rate"`
	Status           string              `json:"status"`
	PriceIncludesTax bool                `json:"price_includes_tax"`
	NetAmount        string              `json:"net_amount"`
	TaxAmount        string              `json:"tax_amount"`
	TotalAmount      string              `json:"total_amount"`
	BaseNetAmount    string              `json:"base_net_amount"`
	BaseTaxAmount    string              `json:"base_tax_amount"`
	BaseTotalAmount  string              `json:"base_total_amount"`
	Remark           string              `json:"remark"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
	Lines            []orderLineResponse `json:"lines"`
}

type receiptLineResponse struct {
	OrderLineID     string `json:"order_line_id"`
	MaterialID      string `json:"material_id"`
	MaterialName    string `json:"material_name"`
	Quantity        string `json:"quantity"`
	UnitPrice       string `json:"unit_price"`
	TaxRateID       string `json:"tax_rate_id"`
	TaxRate         string `json:"tax_rate"`
	Amount          string `json:"amount"`
	TaxAmount       string `json:"tax_amount"`
	GrossAmount     string `json:"gross_amount"`
	BaseAmount      string `json:"base_amount"`
	BaseTaxAmount   string `json:"base_tax_amount"`
	BaseGrossAmount string `json:"base_gross_amount"`
}

type receiptResponse struct {
	ID               string                `json:"id"`
	Code             string                `json:"code"`
	OrderID          string                `json:"order_id"`
	OrderCode        string                `json:"order_code"`
	SupplierName     string                `json:"supplier_name"`
	WarehouseID      string                `json:"warehouse_id"`
	WarehouseName    string                `json:"warehouse_name"`
	ReceiptDate      string                `json:"receipt_date"`
	Status           string                `json:"status"`
	PriceIncludesTax bool                  `json:"price_includes_tax"`
	ExchangeRate     string                `json:"exchange_rate"`
	NetAmount        string                `json:"net_amount"`
	TaxAmount        string                `json:"tax_amount"`
	TotalAmount      string                `json:"total_amount"`
	BaseNetAmount    string                `json:"base_net_amount"`
	BaseTaxAmount    string                `json:"base_tax_amount"`
	BaseTotalAmount  string                `json:"base_total_amount"`
	Remark           string                `json:"remark"`
	PayableID        string                `json:"payable_id"`
	PayableCode      string                `json:"payable_code"`
	PayableDueDate   string                `json:"payable_due_date"`
	VoucherCode      string                `json:"voucher_code"`
	CreatedAt        time.Time             `json:"created_at"`
	Lines            []receiptLineResponse `json:"lines"`
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
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid order payload")
		return
	}
	order, err := h.orders.Create(c.Request.Context(), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toOrderResponse(order)
	h.recordAudit(c, "purchase.order.create", "purchase_order", order.ID, response)
	httpx.Created(c, response)
}

func (h *Handler) updateOrder(c *gin.Context) {
	input, err := parseOrderInput(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid order payload")
		return
	}
	order, err := h.orders.UpdateDraft(c.Request.Context(), c.Param("id"), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toOrderResponse(order)
	h.recordAudit(c, "purchase.order.update", "purchase_order", order.ID, response)
	httpx.OK(c, response)
}

func (h *Handler) submitOrder(c *gin.Context) {
	h.transition(c, "purchase.order.submit", h.orders.Submit)
}

func (h *Handler) approveOrder(c *gin.Context) {
	h.transition(c, "purchase.order.approve", h.orders.Approve)
}

func (h *Handler) cancelOrder(c *gin.Context) {
	h.transition(c, "purchase.order.cancel", h.orders.Cancel)
}

func (h *Handler) transition(c *gin.Context, action string, fn func(ctx context.Context, id string) error) {
	if err := fn(c.Request.Context(), c.Param("id")); err != nil {
		h.handleError(c, err)
		return
	}
	h.recordAudit(c, action, "purchase_order", c.Param("id"), nil)
	httpx.OK(c, gin.H{"id": c.Param("id"), "action": action})
}

func (h *Handler) receiveOrder(c *gin.Context) {
	var request receiveRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid receive payload")
		return
	}
	input, err := parseReceiveInput(request)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid receive payload")
		return
	}
	receipt, err := h.receipts.Receive(c.Request.Context(), c.Param("id"), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toReceiptResponse(receipt)
	h.recordAudit(c, "purchase.receipt.create", "purchase_receipt", receipt.ID, response)
	httpx.Created(c, response)
}

func (h *Handler) listReceipts(c *gin.Context) {
	page, pageSize := parsePage(c)
	receipts, total, err := h.receipts.List(c.Request.Context(), c.Query("order_id"), page, pageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}

	items := make([]receiptResponse, 0, len(receipts))
	for _, receipt := range receipts {
		items = append(items, toReceiptResponse(receipt))
	}
	httpx.OK(c, pagedResponse[receiptResponse]{Items: items, Total: total})
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
			TaxRateID:  line.TaxRateID,
		})
	}
	priceIncludesTax := true
	if request.PriceIncludesTax != nil {
		priceIncludesTax = *request.PriceIncludesTax
	}
	exchangeRate, err := parseOptionalDecimal(request.ExchangeRate)
	if err != nil {
		return app.OrderInput{}, err
	}
	return app.OrderInput{
		SupplierID: request.SupplierID, WarehouseID: request.WarehouseID,
		OrderDate: orderDate, Currency: request.Currency,
		ExchangeRate: exchangeRate, PriceIncludesTax: priceIncludesTax,
		Remark: request.Remark, Lines: lines,
	}, nil
}

func parseReceiveInput(request receiveRequest) (app.ReceiveInput, error) {
	receiptDate, err := parseDate(request.ReceiptDate)
	if err != nil {
		return app.ReceiveInput{}, err
	}
	exchangeRate, err := parseOptionalDecimal(request.ExchangeRate)
	if err != nil {
		return app.ReceiveInput{}, err
	}
	lines := make([]app.ReceiveLineInput, 0, len(request.Lines))
	for _, line := range request.Lines {
		quantity, quantityErr := decimal.NewFromString(strings.TrimSpace(line.Quantity))
		if quantityErr != nil {
			return app.ReceiveInput{}, quantityErr
		}
		lines = append(lines, app.ReceiveLineInput{OrderLineID: line.OrderLineID, Quantity: quantity})
	}
	return app.ReceiveInput{ReceiptDate: receiptDate, ExchangeRate: exchangeRate, Remark: request.Remark, Lines: lines}, nil
}

func parseOptionalDecimal(value string) (decimal.Decimal, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(value)
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
	case errors.Is(err, domain.ErrOverReceive):
		httpx.Error(c, http.StatusConflict, "OVER_RECEIVE", "received quantity exceeds remaining quantity")
	case errors.Is(err, domain.ErrInvalidStatus):
		httpx.Error(c, http.StatusConflict, "INVALID_STATUS", "order status does not allow this operation")
	case errors.Is(err, domain.ErrConflict):
		httpx.Error(c, http.StatusConflict, "CONFLICT", "order or receipt is in conflict")
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
		h.log.Error("purchase request failed",
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
			TaxRateID: line.TaxRateID, TaxRate: line.TaxRate.String(),
			Amount: line.Amount.StringFixed(4), TaxAmount: line.TaxAmount.StringFixed(4),
			GrossAmount: line.GrossAmount.StringFixed(4),
			BaseAmount:  line.BaseAmount.StringFixed(4), BaseTaxAmount: line.BaseTaxAmount.StringFixed(4),
			BaseGrossAmount: line.BaseGrossAmount.StringFixed(4), ReceivedQty: line.ReceivedQty.String(),
			RemainingQty: line.Remaining().String(),
		})
	}
	return orderResponse{
		ID: order.ID, Code: order.Code, SupplierID: order.SupplierID, SupplierName: order.SupplierName,
		WarehouseID: order.WarehouseID, WarehouseName: order.WarehouseName,
		OrderDate: order.OrderDate.Format("2006-01-02"), Currency: order.Currency,
		ExchangeRate: order.ExchangeRate.StringFixed(8),
		Status:       string(order.Status), PriceIncludesTax: order.PriceIncludesTax,
		NetAmount: order.NetAmount.StringFixed(4), TaxAmount: order.TaxAmount.StringFixed(4),
		TotalAmount:   order.TotalAmount.StringFixed(4),
		BaseNetAmount: order.BaseNetAmount.StringFixed(4), BaseTaxAmount: order.BaseTaxAmount.StringFixed(4),
		BaseTotalAmount: order.BaseTotalAmount.StringFixed(4),
		Remark:          order.Remark, CreatedAt: order.CreatedAt, UpdatedAt: order.UpdatedAt, Lines: lines,
	}
}

func toReceiptResponse(receipt *domain.Receipt) receiptResponse {
	lines := make([]receiptLineResponse, 0, len(receipt.Lines))
	for _, line := range receipt.Lines {
		lines = append(lines, receiptLineResponse{
			OrderLineID: line.OrderLineID, MaterialID: line.MaterialID, MaterialName: line.MaterialName,
			Quantity: line.Quantity.String(), UnitPrice: line.UnitPrice.String(),
			TaxRateID: line.TaxRateID, TaxRate: line.TaxRate.String(),
			Amount: line.Amount.StringFixed(4), TaxAmount: line.TaxAmount.StringFixed(4),
			GrossAmount: line.GrossAmount.StringFixed(4),
			BaseAmount:  line.BaseAmount.StringFixed(4), BaseTaxAmount: line.BaseTaxAmount.StringFixed(4),
			BaseGrossAmount: line.BaseGrossAmount.StringFixed(4),
		})
	}
	payableDueDate := ""
	if !receipt.PayableDueDate.IsZero() {
		payableDueDate = receipt.PayableDueDate.Format("2006-01-02")
	}
	return receiptResponse{
		ID: receipt.ID, Code: receipt.Code, OrderID: receipt.OrderID, OrderCode: receipt.OrderCode,
		SupplierName: receipt.SupplierName, WarehouseID: receipt.WarehouseID, WarehouseName: receipt.WarehouseName,
		ReceiptDate: receipt.ReceiptDate.Format("2006-01-02"), Status: receipt.Status,
		PriceIncludesTax: receipt.PriceIncludesTax, ExchangeRate: receipt.ExchangeRate.StringFixed(8),
		NetAmount: receipt.NetAmount.StringFixed(4), TaxAmount: receipt.TaxAmount.StringFixed(4),
		TotalAmount:   receipt.TotalAmount().StringFixed(4),
		BaseNetAmount: receipt.BaseNetAmount.StringFixed(4), BaseTaxAmount: receipt.BaseTaxAmount.StringFixed(4),
		BaseTotalAmount: receipt.BaseTotalAmount.StringFixed(4), Remark: receipt.Remark,
		PayableID: receipt.PayableID, PayableCode: receipt.PayableCode, PayableDueDate: payableDueDate, VoucherCode: receipt.VoucherCode,
		CreatedAt: receipt.CreatedAt, Lines: lines,
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
